package server

import (
	"context"
	"time"
)

func (s *Server) CheckUsers(ctx context.Context) error {
	users, err := s.db.GetUsers(ctx)
	if err != nil {
		return err
	}

	for _, user := range users {
		if user.GetUsername() == "" {
			err = s.UpdateUser(ctx, user)
			if err != nil {
				return err
			}

			err = s.db.SaveUser(ctx, user)
			if err != nil {
				return err
			}
		}

		if time.Since(user.GetLastUpdate()) > time.Hour {
			err = s.UpdateUserCheckins(ctx, user)
			if err != nil {
				return err
			}

			user.LastUpdate = time.Now().Unix()
			err = s.db.SaveUser(ctx, user)
			if err != nil {
				return err
			}
		}
	}
}
