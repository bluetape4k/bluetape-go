# Image Research Ordering

For image work, do not begin by porting all `bluetape4k-image` modules. The Go
value is strongest in bounded thumbnail, resize, and conversion behavior that
can be tested with fixtures, fuzz inputs, and benchmarks without native
dependencies.

libvips is still the likely acceleration path for large images, but it belongs
behind an optional package only after native detection, codec support, runtime
lifecycle, memory behavior, and benchmark deltas are proven in this repo.

CAPTCHA and OCR need service-specific justification. CAPTCHA examples must
state replay, expiry, rate-limit, and OCR-bypass limits; OCR should wait for a
native Tesseract issue with containerized smoke tests.
