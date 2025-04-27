package postgres

func changesetQuery() string {
	return `
	SELECT
		json_build_object(
			'id', c.id,
			'instanceId', c.instance_id,
			'destroy', c.destroy,
			'status', c.status,
			'blueprintLocation', c.blueprint_location,
			'changes', c.changes,
			'created', EXTRACT(EPOCH FROM c.created)::bigint
		) As changeset_json
	FROM changesets c
	WHERE id = @id`
}

func saveChangesetQuery() string {
	return `
		INSERT INTO changesets (
			id,
			instance_id,
			destroy,
			"status",
			blueprint_location,
			"changes",
			created
		) VALUES (
			@id,
			@instanceId,
			@destroy,
			@status,
			@blueprintLocation,
			@changes,
			@created
		)
		ON CONFLICT (id) DO UPDATE SET
			status = excluded.status,
			changes = excluded.changes
	`
}

func cleanupChangesetsQuery() string {
	return `
		DELETE FROM changesets
		WHERE created < @cleanupBefore
	`
}
