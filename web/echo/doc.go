// Package echoadapter provides a thin adapter from bluetape-go web, rate-limit,
// JWT, and resilience contracts to the Echo HandlerFunc boundary.
//
// Echo는 이 package에만 의존하며 framework-neutral core package는 Echo를
// import하지 않는다.
package echoadapter
