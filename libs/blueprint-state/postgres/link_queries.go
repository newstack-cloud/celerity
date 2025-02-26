package postgres

import "github.com/two-hundred/celerity/libs/blueprint/state"

func linkQuery() string {
	return `SELECT json FROM links_json WHERE id = @linkId`
}

func linkByNameQuery() string {
	return `
	SELECT json FROM links_json
	WHERE instance_id = @instanceId AND "name" = @linkName`
}

func updateLinkStatusQuery(statusInfo *state.LinkStatusInfo) string {
	query := `
	UPDATE links
	SET
		status = @status,
		precise_status = @preciseStatus`

	if statusInfo.LastStatusUpdateTimestamp != nil {
		query += `,
		last_status_update_timestamp = @lastStatusUpdateTimestamp`
	}

	if statusInfo.LastDeployedTimestamp != nil {
		query += `,
		last_deployed_timestamp = @lastDeployedTimestamp`
	}

	if statusInfo.LastDeployAttemptTimestamp != nil {
		query += `,
		last_deploy_attempt_timestamp = @lastDeployAttemptTimestamp`
	}

	if statusInfo.Durations != nil {
		query += `,
		durations = @durations`
	}

	if statusInfo.FailureReasons != nil {
		query += `,
		failure_reasons = @failureReasons`
	}

	query += `
	WHERE id = @linkId`

	return query
}

func removeLinkQuery() string {
	return `DELETE FROM links WHERE id = @linkId`
}
