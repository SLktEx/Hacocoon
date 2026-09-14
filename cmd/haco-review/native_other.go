//go:build !windows

package main

import "errors"

func nativeReview(configuration, string, string) error {
	return errors.New("native Windows notifications are required")
}
