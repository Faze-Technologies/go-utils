package request

import "strings"

// SplitAction splits a path parameter of the form "{id}:{action}" (for example "6a1f...:cancel") into its id and
// action, splitting at the last colon. ok is false when there is no colon, or the id or the action is empty.
func SplitAction(param string) (id, action string, ok bool) {
	i := strings.LastIndex(param, ":")
	if i <= 0 || i == len(param)-1 {
		return "", "", false
	}
	return param[:i], param[i+1:], true
}
