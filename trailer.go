package tablo

// A Trailer is one of the four commit trailers of SYNTAX.md. Line returns it
// as Git writes it, with no newline, or a *UsageError when a field is
// missing, extra or malformed.
type Trailer struct {
	Kind  string // "authorised", "reviewed", "reaffirmed" or "model"
	Task  string // for the three task trailers
	Gate  string // for reviewed
	Model string // for model: printable ASCII with no space
}

// Line returns the trailer as one line of a commit message: "Authorised: <id>",
// "Reviewed: <id> <gate>", "Reaffirmed: <id>" or "Model: <identifier>". It
// checks the syntax of each field and reads no project; a task or gate the
// project lacks is for Snapshot.CheckTrailer.
func (t Trailer) Line() (string, error) {
	var head string
	switch t.Kind {
	case "authorised":
		head = "Authorised"
	case "reviewed":
		head = "Reviewed"
	case "reaffirmed":
		head = "Reaffirmed"
	case "model":
		head = "Model"
	default:
		return "", usagef("%q is not a trailer kind: authorised, reviewed, reaffirmed or model", t.Kind)
	}
	if t.Kind == "model" {
		if t.Task != "" || t.Gate != "" {
			return "", usagef("a model trailer names no task or gate")
		}
		if t.Model == "" {
			return "", usagef("a model trailer needs an identifier")
		}
		for i := 0; i < len(t.Model); i++ {
			if c := t.Model[i]; c <= ' ' || c > '~' {
				return "", usagef("the model identifier %q holds a character that is not printable ASCII without a space", t.Model)
			}
		}
		return head + ": " + t.Model, nil
	}
	if t.Model != "" {
		return "", usagef("a %s trailer names no model", t.Kind)
	}
	if t.Task == "" {
		return "", usagef("a %s trailer needs a task id", t.Kind)
	}
	if !idPattern.MatchString(t.Task) {
		return "", usagef("the task id %q is not four lowercase hexadecimal digits", t.Task)
	}
	if t.Kind != "reviewed" {
		if t.Gate != "" {
			return "", usagef("a %s trailer names no gate", t.Kind)
		}
		return head + ": " + t.Task, nil
	}
	if t.Gate == "" {
		return "", usagef("a reviewed trailer needs a gate")
	}
	if !keyPattern.MatchString(t.Gate) {
		return "", usagef("the gate %q is not a gate key", t.Gate)
	}
	return head + ": " + t.Task + " " + t.Gate, nil
}
