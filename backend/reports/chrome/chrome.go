// Package chrome печатает отчёт через headless Chromium.
//
// Chromium живёт отдельным контейнером (chromedp/headless-shell) и доступен
// по remote DevTools (CHROME_WS_URL); в бинарнике его нет. Рендер открывает
// страницу печати фронтенда (/print/report/:id?token=…), ждёт флага
// window.__reportReady, который страница ставит после данных, шрифта и
// отрисовки Recharts, и дальше печатает PDF (Page.printToPDF, размер листа
// и поля задаёт @page в CSS страницы) или снимает графические части секций
// ([data-slide] → [data-slide-image]) в 2× для слайдов PPTX.
//
// Один рендер за раз на процесс: Chromium держит вкладку и память, а
// параллельные вкладки только делят между собой одно ядро контейнера.
package chrome

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/png"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// Slide — снимок графической части одной секции страницы печати.
type Slide struct {
	// Index — порядковый номер секции на странице: совпадает с порядком
	// блоков в запросе отчёта.
	Index int
	Block string
	PNG   []byte
	// Width, Height — размер картинки в пикселях (уже с учётом 2×).
	Width, Height int
}

// Result — что вернул один проход по странице.
type Result struct {
	PDF    []byte
	Slides []Slide
}

// Request — что снять со страницы за один проход.
type Request struct {
	URL    string
	PDF    bool
	Slides bool
}

// Renderer — клиент одного Chromium.
type Renderer struct {
	wsURL       string
	pageTimeout time.Duration
	mu          sync.Mutex
}

const (
	// Ширина окна — лист печатной страницы (273 мм ≈ 1032 px) с полями
	// экранного режима; масштаб 2 даёт чёткий растр на слайде при 200 %.
	viewportWidth  = 1200
	viewportHeight = 900
	deviceScale    = 2
	readyPoll      = 200 * time.Millisecond
)

// New — рендер для Chromium по адресу DevTools (например ws://chromium:9222).
// pageTimeout — сколько ждать готовности страницы.
func New(wsURL string, pageTimeout time.Duration) *Renderer {
	return &Renderer{wsURL: wsURL, pageTimeout: pageTimeout}
}

// Render открывает страницу и снимает запрошенное. Ошибка страницы
// (window.__reportError) и тайм-аут готовности — обычные ошибки; recover
// от паники chromedp здесь не нужен: паник в нём не бывает, а контекст
// с дедлайном закрывает вкладку в любом исходе.
func (r *Renderer) Render(ctx context.Context, req Request) (Result, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(ctx, r.wsURL)
	defer cancelAlloc()
	tab, cancelTab := chromedp.NewContext(allocCtx)
	defer cancelTab()

	var out Result
	if err := chromedp.Run(tab,
		emulation.SetDeviceMetricsOverride(viewportWidth, viewportHeight, deviceScale, false),
		chromedp.Navigate(req.URL),
	); err != nil {
		return out, fmt.Errorf("chromium: открытие страницы: %w", err)
	}
	if err := r.waitReady(tab); err != nil {
		return out, err
	}
	if req.PDF {
		if err := chromedp.Run(tab, chromedp.ActionFunc(func(ctx context.Context) error {
			data, _, err := page.PrintToPDF().
				WithPrintBackground(true).
				WithPreferCSSPageSize(true).
				WithDisplayHeaderFooter(false).
				Do(ctx)
			out.PDF = data
			return err
		})); err != nil {
			return out, fmt.Errorf("chromium: печать PDF: %w", err)
		}
	}
	if req.Slides {
		slides, err := captureSlides(tab)
		if err != nil {
			return out, err
		}
		out.Slides = slides
	}
	return out, nil
}

// waitReady ждёт флаг готовности страницы или её ошибку.
func (r *Renderer) waitReady(tab context.Context) error {
	deadline := time.Now().Add(r.pageTimeout)
	for {
		var state struct {
			Ready bool   `json:"ready"`
			Error string `json:"error"`
		}
		err := chromedp.Run(tab, chromedp.Evaluate(
			`({ ready: window.__reportReady === true, error: window.__reportError || '' })`, &state))
		if err != nil {
			return fmt.Errorf("chromium: опрос готовности: %w", err)
		}
		if state.Error != "" {
			return fmt.Errorf("страница печати: %s", state.Error)
		}
		if state.Ready {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New("страница печати не подготовилась вовремя")
		}
		select {
		case <-tab.Done():
			return fmt.Errorf("chromium: %w", tab.Err())
		case <-time.After(readyPoll):
		}
	}
}

// slideMeta — секции страницы и наличие графической части у каждой.
type slideMeta struct {
	Index    int    `json:"index"`
	Block    string `json:"block"`
	HasImage bool   `json:"hasImage"`
}

func captureSlides(tab context.Context) ([]Slide, error) {
	var metas []slideMeta
	if err := chromedp.Run(tab, chromedp.Evaluate(`[...document.querySelectorAll('[data-slide]')].map((s, i) => ({
		index: i, block: s.getAttribute('data-slide') || '', hasImage: !!s.querySelector('[data-slide-image]'),
	}))`, &metas)); err != nil {
		return nil, fmt.Errorf("chromium: список секций: %w", err)
	}
	slides := make([]Slide, 0, len(metas))
	for _, m := range metas {
		if !m.HasImage {
			continue
		}
		sel := fmt.Sprintf(`[data-slide-index="%d"] [data-slide-image]`, m.Index)
		var buf []byte
		if err := chromedp.Run(tab,
			chromedp.ScrollIntoView(sel, chromedp.ByQuery),
			chromedp.Screenshot(sel, &buf, chromedp.ByQuery, chromedp.NodeVisible),
		); err != nil {
			return nil, fmt.Errorf("chromium: снимок секции %s: %w", m.Block, err)
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(buf))
		if err != nil {
			return nil, fmt.Errorf("chromium: снимок секции %s: %w", m.Block, err)
		}
		slides = append(slides, Slide{Index: m.Index, Block: m.Block, PNG: buf, Width: cfg.Width, Height: cfg.Height})
	}
	return slides, nil
}

// PrintURL собирает адрес страницы печати: база из REPORT_PRINT_BASE_URL
// (http://frontend/print/report) + идентификатор задания + токен.
func PrintURL(base, jobID, token string) string {
	return strings.TrimRight(base, "/") + "/" + jobID + "?token=" + token
}
