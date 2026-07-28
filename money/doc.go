// Package money ISO 4217 통화와 decimal-backed 금액 값을 다룹니다.
//
// 이 package는 govalues/money를 계산 engine으로 사용하지만 public API는
// bluetape-go가 소유합니다. 통화 불일치, invalid zero-value, 환율 오류는
// errors.Is로 확인 가능한 sentinel error로 반환됩니다.
//
// NewExchangeRate와 Convert는 순수 value API입니다. Provider-backed 환율
// 변환은 ExchangeRateProvider와 ConvertWithProvider를 사용하며, network/cache
// failure와 source/freshness metadata를 caller에게 드러냅니다.
package money
