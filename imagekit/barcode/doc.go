// Package barcode는 제한된 QR·Code128 이미지를 생성하는 순수 Go helper를
// 제공한다.
//
// QR 입력은 유효한 UTF-8 문자열을, Code128 입력은 printable ASCII만
// 허용한다. 출력 크기와 픽셀 수에는 고정 상한이 있으며 잘못된 요청은
// provider를 호출하기 전에 거부한다. Render는 표준 image.Image를, EncodePNG는
// PNG 바이트를 반환한다.
//
// 호출자는 context.Context의 취소와 동시 실행 수를 소유한다. provider 호출을
// 이미 시작한 뒤에는 이 helper가 해당 호출을 선점하지 않으며, 확인 가능한
// 경계에서 취소를 확인한다. provider 오류 원문은 공개 오류에 포함하지 않는다.
package barcode
