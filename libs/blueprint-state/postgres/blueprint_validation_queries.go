package postgres

func blueprintValidationQuery() string {
	return `
	SELECT
		json_build_object(
			'id', v.id,
			'blueprintLocation', v.blueprint_location,
			'status', v.status,
			'created', EXTRACT(EPOCH FROM v.created)::bigint
		) As blueprint_validation_json
	FROM blueprint_validations v
	WHERE id = @id`
}

func cleanupBlueprintValidationsQuery() string {
	return `
	DELETE FROM blueprint_validations
	WHERE created < @cleanupBefore`
}

func saveBlueprintValidationQuery() string {
	return `
	INSERT INTO blueprint_validations (
		id,
		blueprint_location,
		"status",
		created
	) VALUES (
		@id,
		@blueprintLocation,
		@status,
		@created
	)
	ON CONFLICT (id) DO UPDATE SET
		status = excluded.status`
}
