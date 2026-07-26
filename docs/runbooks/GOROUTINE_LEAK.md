# Goroutine Leak

Meaning: goroutine count grows after traffic stops.

Check: worker shutdown, tickers, contexts, response bodies, Redis consumers and provider waits.

Mitigate: stop low-priority workers, enforce context timeouts and capture internal-only goroutine profile.

Scale: not applicable until leak is fixed.

Forbidden: do not mask leaks by increasing shutdown timeout only.

Recovery: goroutine count returns near baseline after idle period.
