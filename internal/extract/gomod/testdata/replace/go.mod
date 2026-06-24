module example.com/app

go 1.26

require (
	github.com/go-git/go-billy/v5 v5.9.0
	github.com/incompat/lib v2.1.0+incompatible
)

replace github.com/go-git/go-billy/v5 => ../local-billy
