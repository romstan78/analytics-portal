// Пакетный пересчёт пары «рубли / упаковки» в строках прогноза.
//
// Нужна один раз после перехода на хранение обеих метрик: строки, заведённые
// раньше, держат только введённую единицу, а парная в них пустая. Дальше пару
// поддерживают сами обработчики — прогноза, режима ведения и цен.
//
// Запускать можно и повторно: пересчёт идёт тем же путём, что и чтение формы,
// и на уже согласованных строках ничего не меняет.
package main

import (
	"flag"
	"fmt"
	"os"

	"backend/config"
	"backend/repository"
	"backend/services"
)

func main() {
	year := flag.Int("year", 0, "год; 0 — все годы, где есть прогнозы")
	networkID := flag.Int("network", 0, "сеть; 0 — все активные сети")
	flag.Parse()

	if err := config.Init(); err != nil {
		fmt.Fprintln(os.Stderr, "подключение к БД:", err)
		os.Exit(1)
	}

	targets, err := repository.NetworkYearsWithForecasts(*year)
	if err != nil {
		fmt.Fprintln(os.Stderr, "выбор сетей:", err)
		os.Exit(1)
	}

	var processed, written, failed int64
	for _, target := range targets {
		if *networkID > 0 && target.NetworkID != *networkID {
			continue
		}
		affected, err := services.RebuildForecastPairsYear(target.NetworkID, target.Year)
		if err != nil {
			// Одна сбойная сеть не должна останавливать пересчёт, но и молча
			// пропустить её нельзя: считаем и печатаем.
			fmt.Fprintf(os.Stderr, "сеть %d, год %d: %v\n", target.NetworkID, target.Year, err)
			failed++
			continue
		}
		processed++
		written += affected
	}

	fmt.Printf("пересчитано сетей-лет: %d, строк записано: %d, с ошибкой: %d\n", processed, written, failed)
	if failed > 0 {
		os.Exit(1)
	}
}
