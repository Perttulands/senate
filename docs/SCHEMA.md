# Senate Schema

## Case

Required fields after normalization:

- `id` (string)
- `type` (string)
- `summary` (string)
- `question` (string)
- `filed_at` (RFC3339)

Optional:

- `evidence` ([]string)
- `requested_decision` (string)
- `filed_by` (string)

## Verdict

Required fields:

- `case_id` (string)
- `filed_at` (RFC3339)
- `verdict_at` (RFC3339)
- `type` (string)
- `summary` (string)
- `verdict` (`approved|rejected|amended|deferred`)
- `reasoning` (string)
- `implementation` (string)
- `binding` (bool)
- `judge` (string)

Optional:

- `dissent` (string)
- `handoff` (`system`, `bead_id`, `status`, `created_at`)
