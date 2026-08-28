package repository

import (
	"database/sql"
	"errors"
	"time"

	"backend/config"
	"backend/models"
)

// Реестр фоновых выгрузок интернет-продаж.
//
// Состояние живёт в БД, а не в памяти процесса: после перезапуска опрос
// статуса возвращал 404, и выгрузку приходилось запускать заново, а со второй
// репликой задание терялось бы штатно — клиент опросил бы статус у соседа,
// который о нём не знает.
//
// Правила обслуживания реестра вынесены отдельно от SQL: это и есть логика
// («когда задание считать зависшим, когда просроченным»), и проверять её нужно
// уметь без базы.

// SalesExportJobPolicy — сроки жизни задания.
//
// TTL — сколько готовое задание и его файл остаются доступными. StuckAfter —
// после какого срока задание, не отчитавшееся ни успехом, ни ошибкой,
// считается зависшим: горутина, которая перевела бы его в failed, могла уже
// не отчитаться — процесс перезапустили или задание оборвалось иначе.
type SalesExportJobPolicy struct {
	TTL        time.Duration
	StuckAfter time.Duration
}

// SalesExportJobAction — что делать с заданием при обходе реестра.
type SalesExportJobAction int

const (
	SalesExportJobKeep SalesExportJobAction = iota
	SalesExportJobFail
	SalesExportJobDrop
)

// SalesExportJobCleanup решает судьбу одного задания.
//
// Незавершённое закрывается по StuckAfter, завершённое удаляется по TTL.
// Свежее незавершённое не трогается: большая выгрузка идёт долго, и обрывать
// её на полпути нельзя — в том числе когда её готовит другая реплика.
func SalesExportJobCleanup(job models.SalesExportJob, now time.Time, policy SalesExportJobPolicy) SalesExportJobAction {
	if job.Status == "queued" || job.Status == "running" {
		if now.Sub(job.CreatedAt) >= policy.StuckAfter {
			return SalesExportJobFail
		}
		return SalesExportJobKeep
	}
	if now.Sub(job.CreatedAt) >= policy.TTL {
		return SalesExportJobDrop
	}
	return SalesExportJobKeep
}

func scanSalesExportJob(scan func(dest ...any) error) (models.SalesExportJob, error) {
	var (
		job         models.SalesExportJob
		filePath    sql.NullString
		errorText   sql.NullString
		completedAt sql.NullTime
	)
	err := scan(&job.ID, &job.Owner, &job.Status, &job.TotalRows, &job.FileName,
		&filePath, &errorText, &job.CreatedAt, &completedAt)
	if err != nil {
		return models.SalesExportJob{}, err
	}
	job.FilePath = filePath.String
	job.Error = errorText.String
	if completedAt.Valid {
		job.CompletedAt = completedAt.Time
	}
	return job, nil
}

const salesExportJobColumns = `id, owner_name, status, total_rows, file_name,
	file_path, error_text, created_at, completed_at`

// InsertSalesExportJob заводит задание в состоянии queued.
func InsertSalesExportJob(job models.SalesExportJob) error {
	_, err := config.DB.Exec(
		`INSERT INTO dbo.tbl_SalesExportJobs
		     (id, owner_name, status, total_rows, file_name, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		job.ID, job.Owner, job.Status, job.TotalRows, job.FileName, job.CreatedAt.UTC(),
	)
	return err
}

// GetSalesExportJob читает задание своего владельца.
// Чужое задание не находится: по идентификатору файла хватило бы одной ссылки.
func GetSalesExportJob(id, owner string) (models.SalesExportJob, bool, error) {
	job, err := scanSalesExportJob(config.DB.QueryRow(
		`SELECT `+salesExportJobColumns+`
		   FROM dbo.tbl_SalesExportJobs WHERE id = ? AND owner_name = ?`,
		id, owner,
	).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return models.SalesExportJob{}, false, nil
	}
	if err != nil {
		return models.SalesExportJob{}, false, err
	}
	return job, true, nil
}

// SetSalesExportJobRunning отмечает начало работы.
func SetSalesExportJobRunning(id string) error {
	_, err := config.DB.Exec(
		"UPDATE dbo.tbl_SalesExportJobs SET status = 'running' WHERE id = ?", id)
	return err
}

// SetSalesExportJobReady сохраняет путь готового файла.
func SetSalesExportJobReady(id, filePath string, completedAt time.Time) error {
	_, err := config.DB.Exec(
		`UPDATE dbo.tbl_SalesExportJobs
		    SET status = 'ready', file_path = ?, completed_at = ?, error_text = NULL
		  WHERE id = ?`,
		filePath, completedAt.UTC(), id)
	return err
}

// SetSalesExportJobFailed закрывает задание с причиной.
//
// Причина — короткий текст для пользователя: подробности ошибки остаются в
// логе, в ответе они раскрывают устройство запроса и ничем не помогают.
func SetSalesExportJobFailed(id, message string, completedAt time.Time) error {
	_, err := config.DB.Exec(
		`UPDATE dbo.tbl_SalesExportJobs
		    SET status = 'failed', error_text = ?, completed_at = ?
		  WHERE id = ?`,
		message, completedAt.UTC(), id)
	return err
}

// DeleteSalesExportJob убирает задание из реестра.
func DeleteSalesExportJob(id string) error {
	_, err := config.DB.Exec("DELETE FROM dbo.tbl_SalesExportJobs WHERE id = ?", id)
	return err
}

// ListSalesExportJobs отдаёт весь реестр: он мал по устройству — задания живут
// час, а их число ограничено числом одновременно работающих людей.
func ListSalesExportJobs() ([]models.SalesExportJob, error) {
	rows, err := config.DB.Query(`SELECT ` + salesExportJobColumns + ` FROM dbo.tbl_SalesExportJobs`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []models.SalesExportJob
	for rows.Next() {
		job, scanErr := scanSalesExportJob(rows.Scan)
		if scanErr != nil {
			return nil, scanErr
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}
