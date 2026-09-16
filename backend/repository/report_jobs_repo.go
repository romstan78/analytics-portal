package repository

import (
	"database/sql"
	"errors"
	"time"

	"backend/config"
	"backend/models"
)

// Реестр фоновых заданий на отчёты PDF/PPTX (dbo.tbl_ReportJobs).
//
// Устройство то же, что у Excel-выгрузок продаж: состояние в БД, файл в
// каталоге выгрузок, сроки жизни — SalesExportJobPolicy. Правила «зависшее/
// просроченное» общие, поэтому здесь они не дублируются, а переиспользуются.

// ReportJobCleanup решает судьбу одного задания по тем же правилам, что и
// SalesExportJobCleanup: незавершённое закрывается по StuckAfter,
// завершённое удаляется по TTL.
func ReportJobCleanup(job models.ReportJob, now time.Time, policy SalesExportJobPolicy) SalesExportJobAction {
	return SalesExportJobCleanup(models.SalesExportJob{Status: job.Status, CreatedAt: job.CreatedAt}, now, policy)
}

const reportJobColumns = `id, owner_name, status, format, title, file_name,
	file_path, error_text, created_at, completed_at`

func scanReportJob(scan func(dest ...any) error) (models.ReportJob, error) {
	var (
		job         models.ReportJob
		filePath    sql.NullString
		errorText   sql.NullString
		completedAt sql.NullTime
	)
	err := scan(&job.ID, &job.Owner, &job.Status, &job.Format, &job.Title, &job.FileName,
		&filePath, &errorText, &job.CreatedAt, &completedAt)
	if err != nil {
		return models.ReportJob{}, err
	}
	job.FilePath = filePath.String
	job.Error = errorText.String
	if completedAt.Valid {
		job.CompletedAt = completedAt.Time
	}
	return job, nil
}

// InsertReportJob заводит задание в состоянии queued вместе со снимком.
func InsertReportJob(job models.ReportJob) error {
	_, err := config.DB.Exec(
		`INSERT INTO dbo.tbl_ReportJobs
		     (id, owner_name, status, format, title, file_name, snapshot_json, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID, job.Owner, job.Status, job.Format, job.Title, job.FileName, job.SnapshotJSON, job.CreatedAt.UTC(),
	)
	return err
}

// GetReportJob читает задание своего владельца; чужое не находится.
// Снимок не читается: он нужен только горутине, которая готовит файл.
func GetReportJob(id, owner string) (models.ReportJob, bool, error) {
	job, err := scanReportJob(config.DB.QueryRow(
		`SELECT `+reportJobColumns+` FROM dbo.tbl_ReportJobs WHERE id = ? AND owner_name = ?`,
		id, owner,
	).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return models.ReportJob{}, false, nil
	}
	if err != nil {
		return models.ReportJob{}, false, err
	}
	return job, true, nil
}

// CountActiveReportJobs — сколько заданий владельца ещё готовится.
func CountActiveReportJobs(owner string) (int, error) {
	var n int
	err := config.DB.QueryRow(
		`SELECT COUNT(*) FROM dbo.tbl_ReportJobs
		  WHERE owner_name = ? AND status IN ('queued', 'running')`, owner,
	).Scan(&n)
	return n, err
}

func SetReportJobRunning(id string) error {
	_, err := config.DB.Exec("UPDATE dbo.tbl_ReportJobs SET status = 'running' WHERE id = ?", id)
	return err
}

func SetReportJobReady(id, filePath string, completedAt time.Time) error {
	_, err := config.DB.Exec(
		`UPDATE dbo.tbl_ReportJobs
		    SET status = 'ready', file_path = ?, completed_at = ?, error_text = NULL
		  WHERE id = ?`,
		filePath, completedAt.UTC(), id)
	return err
}

// SetReportJobFailed закрывает задание коротким текстом для пользователя;
// подробности остаются в логе.
func SetReportJobFailed(id, message string, completedAt time.Time) error {
	_, err := config.DB.Exec(
		`UPDATE dbo.tbl_ReportJobs
		    SET status = 'failed', error_text = ?, completed_at = ?
		  WHERE id = ?`,
		message, completedAt.UTC(), id)
	return err
}

func DeleteReportJob(id string) error {
	_, err := config.DB.Exec("DELETE FROM dbo.tbl_ReportJobs WHERE id = ?", id)
	return err
}

// ListReportJobs отдаёт весь реестр без снимков: он мал по устройству —
// задания живут час.
func ListReportJobs() ([]models.ReportJob, error) {
	rows, err := config.DB.Query(`SELECT ` + reportJobColumns + ` FROM dbo.tbl_ReportJobs`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []models.ReportJob
	for rows.Next() {
		job, scanErr := scanReportJob(rows.Scan)
		if scanErr != nil {
			return nil, scanErr
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}
