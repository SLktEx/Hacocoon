package gitrepo

import "os"

func (s *RepositoryService) removeNewWorkspaceRecord(object Object) error {
	if err := os.Remove(s.path("work", object.ID)); err != nil {
		return err
	}
	return syncDir(s.Root)
}
