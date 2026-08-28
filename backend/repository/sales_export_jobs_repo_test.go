package repository

import (
	"testing"
	"time"

	"backend/models"
)

var testExportPolicy = SalesExportJobPolicy{TTL: time.Hour, StuckAfter: 30 * time.Minute}

func TestSalesExportJobCleanup(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name string
		job  models.SalesExportJob
		want SalesExportJobAction
	}{
		{
			// Горутина, которая перевела бы задание в failed, уже не отчитается:
			// процесс перезапустили или задание оборвалось иначе.
			name: "зависшее закрывается",
			job:  models.SalesExportJob{Status: "running", CreatedAt: now.Add(-testExportPolicy.StuckAfter - time.Minute)},
			want: SalesExportJobFail,
		},
		{
			// Большая выгрузка идёт долго — в том числе на соседней реплике.
			name: "идущую выгрузку не трогаем",
			job:  models.SalesExportJob{Status: "running", CreatedAt: now.Add(-time.Minute)},
			want: SalesExportJobKeep,
		},
		{
			name: "задание в очереди ждёт своего срока",
			job:  models.SalesExportJob{Status: "queued", CreatedAt: now.Add(-time.Minute)},
			want: SalesExportJobKeep,
		},
		{
			name: "готовое уходит вместе с файлом по TTL",
			job:  models.SalesExportJob{Status: "ready", CreatedAt: now.Add(-testExportPolicy.TTL - time.Minute)},
			want: SalesExportJobDrop,
		},
		{
			name: "свежее готовое остаётся",
			job:  models.SalesExportJob{Status: "ready", CreatedAt: now.Add(-time.Minute)},
			want: SalesExportJobKeep,
		},
		{
			name: "провалившееся тоже живёт до TTL",
			job:  models.SalesExportJob{Status: "failed", CreatedAt: now.Add(-time.Minute)},
			want: SalesExportJobKeep,
		},
		{
			// Иначе причина отказа пропала бы раньше, чем клиент её прочитал.
			name: "провалившееся убирается по TTL",
			job:  models.SalesExportJob{Status: "failed", CreatedAt: now.Add(-testExportPolicy.TTL - time.Minute)},
			want: SalesExportJobDrop,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SalesExportJobCleanup(tc.job, now, testExportPolicy); got != tc.want {
				t.Fatalf("SalesExportJobCleanup(%+v) = %v, ожидалось %v", tc.job, got, tc.want)
			}
		})
	}
}
