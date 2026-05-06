package main

import "github.com/heartleo/subtrans/internal/cli"

var version = "dev"

func main() {
	cli.Execute(version)
}
