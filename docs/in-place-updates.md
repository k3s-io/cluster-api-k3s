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
- Unsupported complete diffs use Machine replacement.
- `maxSurge: 0` can therefore become delete-first replacement.
- Delete-first replacement can cause control-plane and API downtime and can
  lose quorum, especially for a one-replica control plane.
- CACP3 orchestrates the update but does not mutate the host or K3s binary. A
  production Runtime Extension implementing `UpdateMachine` performs that
  mutation.

## Known limitations

- Only one in-place update extension is supported.
- Automatic rollback is not implemented.
- After update-in-progress is set, a resumed handoff recomputes the latest desired state without repeating `CanUpdateMachine`, matching KCP v1.12.9.
