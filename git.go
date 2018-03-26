package main

import (
	"gopkg.in/src-d/go-git.v4"
	log "github.com/sirupsen/logrus"
)

func checkout(repo string, destDir string, logger *log.Logger) error {

	logger.WithFields(log.Fields{
		"repo": repo,
		"dest": destDir,
	}).Info("GIT: Cloning...")

	_, err := git.PlainClone(destDir, false, &git.CloneOptions{
		URL: repo,
		Progress: logger.Writer(),
	})
	return err
}