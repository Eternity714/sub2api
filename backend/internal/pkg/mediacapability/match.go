package mediacapability

import "encoding/json"

// MatchCandidate checks accepted request semantics without changing the original request.
// The candidate's n limits apply to the whole request; only Batch describes native batching.
func MatchCandidate(normalized *NormalizedRequest, candidate *Compiled) CandidateMatch {
	result := CandidateMatch{Errors: make([]FieldError, 0)}
	if normalized == nil || candidate == nil {
		result.Errors = append(result.Errors, fieldError("$", "invalid_candidate", "normalized request and candidate are required"))
		return result
	}
	result.NativeBatchSize = candidate.schema.Batch.NativeMaxResults
	if normalized.mediaType != "" && normalized.mediaType != candidate.mediaType {
		result.Errors = append(result.Errors, fieldError("$", "media_mismatch", "candidate media type does not match request"))
		return result
	}
	raw, err := json.Marshal(normalized.Values)
	if err != nil {
		result.Errors = append(result.Errors, fieldError("$", "invalid_request", "normalized request cannot be encoded"))
		return result
	}
	matched, errs := candidate.Validate(raw)
	if len(errs) > 0 {
		result.Errors = errs
		return result
	}
	// A different candidate default is allowed only if it does not add or change accepted parameters.
	if !jsonEqual(normalized.Values, matched.Values) {
		result.Errors = append(result.Errors, fieldError("$", "parameter_mismatch", "candidate defaults would change accepted request semantics"))
		return result
	}
	count := normalized.RequestedCount
	if count < 1 {
		count = 1
	}
	if count > result.NativeBatchSize {
		if !candidate.schema.Batch.AllowSplit || candidate.schema.Batch.SemanticallyLinked {
			result.Errors = append(result.Errors, fieldError("n", "unsupported_batch", "candidate cannot preserve this request through independent splitting"))
			return result
		}
		result.NeedsSplit = true
	}
	result.Supported = true
	return result
}
