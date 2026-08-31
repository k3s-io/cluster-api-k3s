# In-place updates

## Requirements

- Cluster API v1.12+
- Core CAPI manager: `RuntimeSDK=true` and `InPlaceUpdates=true`
- K3s control-plane manager: `InPlaceUpdates=true`
- Exactly one Runtime Extension implementing `CanUpdateMachine` and `UpdateMachine`

## Rollout configuration

```yaml
rolloutStrategy:
  type: RollingUpdate
  rollingUpdate:
    maxSurge: 0
```

## Safety and fallback

- `maxSurge` defaults to `1`.
- The top-level `rolloutStrategy` is retained for K3s API compatibility; CAPI v1beta2 uses a nested `rollout.strategy` shape.
- Allowing `maxSurge: 0` below three replicas while `InPlaceUpdates` is enabled is an intentional K3s divergence from official CAPI v1.12.9 validation.
- If the provider restarts with `InPlaceUpdates` disabled, an object previously
  admitted with `maxSurge: 0` and fewer than three replicas remains writable as
  long as its effective replica count and `maxSurge` are unchanged. New
  transitions into that configuration remain rejected.
- Unsupported or ineligible changes normally fall back to Machine replacement.
- With `maxSurge: 0` and fewer than three desired replicas, fallback is blocked
  when the current Machine count is at or below the desired count. Existing
  Machines are left untouched. Enable or fix a working in-place update
  extension, or set `maxSurge: 1` to allow replacement.
- This admission exception does not relax runtime rollout safety: replacement
  remains blocked in the unsafe low-replica zero-surge configuration.
- Fallback remains allowed with at least three desired replicas or when the
  current Machine count exceeds the desired count, allowing surplus deletion.
- An allowed `maxSurge: 0` fallback can become delete-first replacement.
- Delete-first replacement can cause control-plane and API downtime and can
  lose quorum.
- CACP3 orchestrates the update but does not mutate the host or K3s binary. A
  production Runtime Extension implementing `UpdateMachine` performs that
  mutation.

## Known limitations

- Only one in-place update extension is supported.
- Automatic rollback is not implemented.
- After update-in-progress is set, a resumed handoff recomputes the latest desired state without repeating `CanUpdateMachine`, matching KCP v1.12.9.
