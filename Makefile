.PHONY: work-tidy
work-tidy:
	# tidy all modules
	@dirs="$$(go work edit -json | jq -r '.Use[].DiskPath')"; \
	if [ -z "$$dirs" ]; then \
		echo "No workspace modules found; skipping tidy."; \
	else \
		for dir in $$dirs; do \
			(cd "$$dir" && go mod tidy); \
		done; \
	fi

	# sync workspace dependencies
	@go work sync
