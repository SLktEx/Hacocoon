# GitHub Git capability plugin

This package implements the optional GitHub-aware Git capability used by `haco git push`.

It is an adapter/plugin, not a Core domain dependency. Core owns only generic capability contracts and policy decisions. This package owns Git/GitHub-specific behavior such as remote parsing, repository/branch authority checks, stale approval detection, and the final brokered `git push`.

The sandbox does not receive GitHub credentials from this plugin. A push is executed only after the capability service evaluates the exact GitHub repository, target ref, source commit, and force-push semantics.

Hacocoon currently uses ordinary Go package boundaries and static composition for plugins. This is intentionally not a dynamic shared-object/plugin loader.
