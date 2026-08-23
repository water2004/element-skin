package account

import "log"

func reportPostCommitError(operation string, err error) {
	if err != nil {
		log.Printf("account post-commit side effect failed (%s): %v", operation, err)
	}
}
