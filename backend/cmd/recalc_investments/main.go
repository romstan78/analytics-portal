// Пакетный пересчёт расчётных колонок инвестиций.
//
// Экраны портала считают инвестиции на лету и в этой команде не нуждаются.
// Нужна она внешним потребителям — ежедневной выгрузке, BI и интеграциям,
// которые читают колонки tbl_NetworkPlans напрямую. Запускать после заливки
// факта из внешней БД: сама заливка правило применить не может, потому что
// порог смотрит на валовый пул и на правила совместного зачёта.
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
	year := flag.Int("year", 0, "год; 0 — все годы, где есть планы")
	networkID := flag.Int("network", 0, "сеть; 0 — все активные сети")
	flag.Parse()

	if err := config.Init(); err != nil {
		fmt.Fprintln(os.Stderr, "подключение к БД:", err)
		os.Exit(1)
	}

	targets, err := repository.NetworkYearsWithPlans(*year)
	if err != nil {
		fmt.Fprintln(os.Stderr, "выбор сетей:", err)
		os.Exit(1)
	}

	var processed, failed int
	for _, target := range targets {
		if *networkID > 0 && target.NetworkID != *networkID {
			continue
		}
		if err := services.RebuildNetworkInvestmentColumns(target.NetworkID, target.Year); err != nil {
			// Одна сбойная сеть не должна останавливать ночной пересчёт:
			// молчаливо пропустить её тоже нельзя, поэтому считаем и печатаем.
			fmt.Fprintf(os.Stderr, "сеть %d, год %d: %v\n", target.NetworkID, target.Year, err)
			failed++
			continue
		}
		processed++
	}

	fmt.Printf("пересчитано: %d, с ошибкой: %d\n", processed, failed)
	if failed > 0 {
		os.Exit(1)
	}
}
