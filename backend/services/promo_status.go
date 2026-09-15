package services

// Статус промо не вводится руками — он выводится из согласований и факта.
// Сам вывод живёт в SQL (repository.promoStatusCaseSQL), потому что одно и
// то же правило нужно трём путям записи и разовой миграции 034; здесь —
// имена значений и проверка, когда КАМу можно вносить факт.
const (
	PromoStatusInApproval = "В процессе согласования" // по умолчанию для новых
	PromoStatusFinalized  = "Финализировано"          // оба согласования получены
	PromoStatusDone       = "Проведено"               // финализировано + внесён факт
	PromoStatusRejected   = "Отклонено"               // оба согласования отклонены
)

// PromoFactEditable — можно ли вносить фактические показатели: только после
// финализации. У «Проведено» факт остаётся правимым — его уточняют.
func PromoFactEditable(status string) bool {
	return status == PromoStatusFinalized || status == PromoStatusDone
}

// PromoFactChanged — изменились ли поля блока «Фактические показатели», которые
// вводит КАМ. Расчётные фактические поля не смотрим: они следуют за этими.
func PromoFactChanged(old, updated *PromoInputDTO) bool {
	return old.ActualPromoSalesUnits != updated.ActualPromoSalesUnits ||
		old.ActualInvestments != updated.ActualInvestments ||
		old.ActualExternalEcomUnits != updated.ActualExternalEcomUnits ||
		old.ActualCorrectedBaseline != updated.ActualCorrectedBaseline
}

// PromoFactProvided — заполнено ли хоть одно поле факта, вводимое КАМом.
func PromoFactProvided(dto *PromoInputDTO) bool {
	return dto.ActualPromoSalesUnits != 0 || dto.ActualInvestments != 0 ||
		dto.ActualExternalEcomUnits != 0 || dto.ActualCorrectedBaseline != 0
}
