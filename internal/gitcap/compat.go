package gitcap

import githubplugin "github.com/SLktEx/Hacocoon/modules/capability/github"

// Deprecated: the GitHub Git capability implementation lives in
// modules/capability/github. This package is a compatibility facade for
// existing internal callers while composition migrates to the plugin package.
const GitHubCapability = githubplugin.GitHubCapability

type EnvironmentStore = githubplugin.EnvironmentStore
type PushSpec = githubplugin.PushSpec
type Broker = githubplugin.Broker
type Provider = githubplugin.Provider
type GitHubRepository = githubplugin.GitHubRepository

var NewBroker = githubplugin.NewBroker
var NewProvider = githubplugin.NewProvider
var ParseGitHubRemote = githubplugin.ParseGitHubRemote
