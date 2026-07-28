# Image Research Ordering 교훈

image 작업은 모든 `bluetape4k-image` module을 port하는 것으로 시작하지 않는다. Go value는
native dependency 없이 fixture, fuzz input, benchmark로 test할 수 있는 bounded thumbnail,
resize, conversion behavior에서 가장 강하다.

libvips는 여전히 large image의 유력한 acceleration path지만, 이 repo에서 native
detection, codec support, runtime lifecycle, memory behavior, benchmark delta가 증명된
뒤 optional package 뒤에 둬야 한다.

CAPTCHA와 OCR에는 service-specific justification이 필요하다. CAPTCHA example은 replay,
expiry, rate-limit, OCR-bypass limit를 명시해야 한다. OCR은 containerized smoke test를
갖춘 native Tesseract issue를 기다려야 한다.
