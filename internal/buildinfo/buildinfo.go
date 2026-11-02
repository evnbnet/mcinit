package buildinfo

import "fmt"

var Version = "dev"

const Repo = "https://github.com/evanb/mcinit"

func UserAgent() string {
	return fmt.Sprintf("mcinit/%s (+%s)", Version, Repo)
}
