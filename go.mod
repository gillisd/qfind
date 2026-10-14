module qfind

go 1.27

ignore ./references

require (
	github.com/charlievieth/fastwalk v1.0.14
	golang.org/x/sys v0.48.0
)

require golang.org/x/tools v0.50.0 // indirect

tool golang.org/x/tools/cmd/goyacc
