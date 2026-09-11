package sdtd

import "errors"

func asAPI(err error, target **APIError) bool { return errors.As(err, target) }
