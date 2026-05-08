# Tests

This directory contains integration, CLI, parity, benchmark tests, and shared
fixtures under `testdata/`.

Package-local unit tests remain next to internal packages when they need access
to unexported implementation details. Those tests also read fixtures from
`tests/testdata`.
