package state

import "context"

// catalogTransaction serializes catalog observations and atomic mutations.
func (s *EnvironmentJSONStore) catalogTransaction(ctx context.Context, change func(*environmentFileState) (bool, error)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockEnvironmentState(s.path)
	if err != nil {
		return err
	}
	defer unlock()
	data, err := s.readEnvironments()
	if err != nil {
		return err
	}
	dirty, err := change(&data)
	if err != nil {
		return err
	}
	if !dirty {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.writeEnvironments(data)
}
