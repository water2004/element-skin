package identity

import "log"

func reportPostCommitError(operation string, err error) {
	if err != nil {
		log.Printf("identity post-commit side effect failed (%s): %v", operation, err)
	}
}
