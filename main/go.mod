module gorium/main

go 1.23.2

require gorium/cli v0.0.0-00010101000000-000000000000

require (
	golang.org/x/sys v0.25.0
	golang.org/x/term v0.24.0

)

replace gorium/cli => ../cli
