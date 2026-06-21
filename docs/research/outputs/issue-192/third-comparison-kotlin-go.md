| Workload | Go ns/id | Kotlin ns/id | Lower | Note |
|---|---:|---:|---|---|
| Snowflake single | 11.98 | 244.02 | Go | Go synthetic clock; Kotlin real clock/batch ceiling |
| Snowflake concurrent | 90.01 | 243.65 | Go | Go synthetic clock; Kotlin real clock/batch ceiling |
| ULID monotonic single | 64.96 | 26.28 | Kotlin |  |
| ULID monotonic concurrent | 194.80 | 336.00 | Go |  |
| KSUID seconds single | 215.60 | 175.86 | Kotlin |  |
| KSUID seconds concurrent | 256.40 | 242.50 | Kotlin |  |
| KSUID millis single | 123.50 | 167.07 | Go | Kotlin full candidate 3 run; targeted repeat was 157.03 ns/id |
| KSUID millis concurrent | 215.90 | 215.81 | Kotlin |  |
