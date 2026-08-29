# Instance lifecycle

## States

```
pending  (not used — instances are created running)
running  → terminated
```

## Launch

`POST /instances` on the registry picks a healthy compute node and creates an
`i-N` record. The sequence number is loaded from the DB on restart so IDs
never repeat.

## Workspace

The ec2-agent polls `GET /instances?node_id=<id>` every 3 s. When it sees a
new `running` instance it creates:

```
$TEMP/tinyaws/<instance-id>/
```

Jobs submitted with `instance_id` run inside that directory. Deploy jobs
extract their zip there and run `start.ps1` (Windows) or `start.sh` (Linux).

## Process isolation

- **Unix**: each job subprocess is spawned in a new process group (`setpgid`).
- **Windows**: best-effort; no Job Object attached yet.

## Terminate

`DELETE /instances/<id>` sets status to `terminated`. The agent stops
accepting new jobs for that instance on the next poll. The controller
(`control-plane/controller`) reconciles every 15 s and removes the workspace
directory.

## Sequence

```
1. POST /instances            → i-N created (status=running, node_id=X)
2. agent poll: sees i-N       → creates $TEMP/tinyaws/i-N/
3. POST /jobs instance_id=i-N → job assigned to node X
4. agent picks job            → runs in $TEMP/tinyaws/i-N/
5. DELETE /instances/i-N      → status=terminated
6. agent poll: no running     → stops accepting jobs
7. controller reconcile       → removes $TEMP/tinyaws/i-N/
```
