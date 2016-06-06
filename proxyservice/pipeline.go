package proxyservice

import "log"

func TriggerPipeline(hookData *DockerHubWebhookData) error {
	log.Printf("Trigger pipeline for: %s with tag: %s", hookData.Repository.RepoName, hookData.PushData.Tag)
	return nil
}
