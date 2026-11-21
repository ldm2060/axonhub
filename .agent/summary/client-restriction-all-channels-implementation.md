# Client Restriction: Apply to All Channel Types

## RED Phase Summary

6 tests failed as expected, all on non-coding channels (openai) that previously bypassed restrictions:

- `Lenient_rejects_ordinary_browser_client_on_openai` (expected false, got true)
- `Lenient_rejects_random_client_on_openai` (expected false, got true)
- `Strict_rejects_ordinary_browser_client_on_openai` (expected false, got true)
- `Strict_rejects_random_client_on_openai` (expected false, got true)
- `Channel-level_lenient_override_on_openai_rejects_ordinary_clients_when_global_is_off` (expected false, got true)

Root cause: `CheckClientRestriction` had `if !objects.IsCodingChannel(channelType) { return true }` guard that exempted non-coding channels from all restriction checks.

## GREEN Phase Summary

All targeted tests pass (21 test functions, 0 failures):

- `TestClientDetector_DetectClient` - PASS
- `TestClientDetector_IsLenientClientAllowed` - PASS
- `TestClientDetector_IsStrictClientAllowed` - PASS (updated for fallback behavior)
- `TestClientDetector_EdgeCases` - PASS
- `TestClientRestrictionChecker_CheckClientRestriction` - PASS
- `TestClientRestrictionChecker_GetRejectionReason` - PASS
- All ClientRestrictionLevel serialization tests - PASS

## Changed Files

1. `internal/server/biz/client_restriction_checker.go`
   - Removed `objects` import
   - Removed `!objects.IsCodingChannel(channelType)` guard (3 lines)
   - Updated comment to describe global restriction behavior
   - Changed unmapped-channel strict rejection message from "no allowed clients defined" to same lenient message

2. `internal/server/biz/client_detector.go`
   - `IsStrictClientAllowed`: for unmapped channel types, falls back to `IsLenientClientAllowed` instead of returning `false`

3. `internal/server/biz/client_restriction_checker_test.go`
   - Replaced 5 old non-coding-channel test cases with 13 new cases covering lenient/strict/override on openai channel
   - Updated GetRejectionReason test for new unmapped strict message

4. `internal/server/biz/client_detector_test.go`
   - Updated "Unknown channel type" test: supported client now returns true (fallback)
   - Added "Unknown channel type with unsupported client still rejected" test

## Test Coverage

All requested scenarios verified:

- Lenient rejects ordinary browser/random clients on openai
- Lenient allows supported coding clients on openai
- Strict rejects ordinary browser/random clients on openai
- Strict allows supported coding clients on openai via fallback
- Channel-level off override still allows on openai when global strict would reject
- Channel-level lenient override on openai rejects ordinary clients when global is off
- Existing strict same-family behavior for claudecode remains intact

## Concerns

- The `objects.IsCodingChannel` function and `CodingChannelTypes` map are no longer referenced from client_restriction_checker.go but remain in the codebase (used elsewhere). No cleanup needed for this task.
- The strict fallback to lenient for unmapped channels means strict mode on unmapped types (like openai) is effectively lenient. This is intentional per the task spec: strict should not reject every supported coding client on unmapped types.
