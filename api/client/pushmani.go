package client

import (
	"fmt"
	"net/url"

	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/registry"
)

// CmdPushmani pushes an image manifest to the registry.
//
// Usage: docker pushmani NAME[:TAG]
func (cli *DockerCli) CmdPushmani(args ...string) error {
	cmd := Cli.Subcmd("pushmani", []string{"NAME[:TAG]"}, Cli.DockerCommands["pushmani"].Description, true)

	addTrustedFlags(cmd, false)
	cmd.Require(flag.Exact, 1)

	cmd.ParseFlags(args, true)

	remote, tag := parsers.ParseRepositoryTag(cmd.Arg(0))

	// Resolve the Repository name from fqn to RepositoryInfo
	repoInfo, err := registry.ParseRepositoryInfo(remote)
	if err != nil {
		return err
	}
	// Resolve the Auth config relevant for this server
	authConfig := registry.ResolveAuthConfig(cli.configFile, repoInfo.Index)

	if repoInfo.Official {
		username := authConfig.Username
		if username == "" {
			username = "<user>"
		}
		return fmt.Errorf("You cannot push manifest to a \"root\" repository. Please rename your repository to <user>/<repo> (ex: %s/%s)", username, repoInfo.LocalName)
	}

	if isTrusted() {
		return cli.trustedPush(repoInfo, tag, authConfig)
	}

	v := url.Values{}
	v.Set("tag", tag)

	_, _, err = cli.clientRequestAttemptLogin("POST", "/images/"+remote+"/pushmani?"+v.Encode(), nil, cli.out, repoInfo.Index, "pushmani")
	return err
}
