package oauth

import "log"

func reportPostCommitError(operation string, err error) {
	if err != nil {
		log.Printf("oauth post-commit side effect failed (%s): %v", operation, err)
	}
}
