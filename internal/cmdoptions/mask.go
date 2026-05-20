package cmdoptions

import "strings"

// setFlags are helm flags whose values may contain secrets.
var setFlags = []string{"--set", "--set-string", "--set-file", "--set-json"}

// MaskSetValues replaces the value portion of --set key=value arguments with "***".
// It handles both "--set key=val" (space-separated) and "--set=key=val" (equals-joined) forms.
func MaskSetValues(command string) string {
	tokens := tokenize(command)
	for i := 0; i < len(tokens); i++ {
		for _, flag := range setFlags {
			// Form: --set=key=value
			if strings.HasPrefix(tokens[i], flag+"=") {
				tokens[i] = maskToken(tokens[i], len(flag)+1)
				break
			}
			// Form: --set key=value
			if tokens[i] == flag && i+1 < len(tokens) {
				i++
				tokens[i] = maskValues(tokens[i])
				break
			}
		}
	}
	return strings.Join(tokens, " ")
}

// maskToken masks values in a token like "--set=key=val,key2=val2" starting from offset.
func maskToken(token string, offset int) string {
	return token[:offset] + maskValues(token[offset:])
}

// maskValues masks the value portion of comma-separated key=value pairs.
func maskValues(s string) string {
	var b strings.Builder
	for i, pair := range strings.Split(s, ",") {
		if i > 0 {
			b.WriteByte(',')
		}
		if idx := strings.IndexByte(pair, '='); idx >= 0 {
			b.WriteString(pair[:idx+1])
			b.WriteString("***")
		} else {
			b.WriteString(pair)
		}
	}
	return b.String()
}

// tokenize splits a command string respecting single and double quotes.
func tokenize(s string) []string {
	var tokens []string
	var current strings.Builder
	inSingle := false
	inDouble := false

	for i := range len(s) {
		c := s[i]
		switch {
		case c == '\'' && !inDouble:
			inSingle = !inSingle
			current.WriteByte(c)
		case c == '"' && !inSingle:
			inDouble = !inDouble
			current.WriteByte(c)
		case c == ' ' && !inSingle && !inDouble:
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(c)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}
