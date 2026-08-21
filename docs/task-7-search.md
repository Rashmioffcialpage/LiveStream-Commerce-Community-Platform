# Task 7 — search-service (OpenSearch)

Search by creator, category, stream title, and tags, per the spec.
`search-service` owns no writes of its own -- it's a pure reactor to
Kafka events from `stream-service`, keeping one OpenSearch document per
channel in sync.

## Endpoint

| Method | Path | Auth | Notes |
|---|---|---|---|
| GET | `/healthz` | — | |
| GET | `/search?q=` | — | multi-field match across name, creator, category, stream titles, tags |

## What feeds the index

`stream-service` had no Kafka producer before this task except the one
`stream-events` topic added in Task 6 (for `stream-started`). This task
extended it:

| Kafka topic | Event | Fires on | search-service action |
|---|---|---|---|
| `channel-events` (new) | `channel-created` | `POST /channels` succeeding | `IndexChannel` — creates the document |
| `stream-events` | `stream-created` (new event type on the existing topic) | `POST /channels/{slug}/streams` succeeding | `AppendStreamTitle` — partial update via a Painless script |

`stream-events` already carried `stream-started` (Task 6); adding
`stream-created` as a second event type on the same topic rather than a
third topic was safe because notification-service's consumer already
ignores any type it doesn't recognize (`if e.Type != "stream-started" {
return nil }`) -- exactly the kind of forward-compatible event handling
that makes reusing a topic for a related-but-different event safe.

## Design notes

- **`stream-service`'s Kafka producer went from single-topic to
  multi-topic.** It previously had a `Topic` fixed on the `kafka.Writer`
  at construction (fine when there was only one destination). Adding
  `channel-events` meant refactoring `Produce` to take a topic per call
  instead -- the correct shape for "one process publishes to more than
  one topic," not a workaround.
- **Partial update via Painless, not read-modify-write in Go.** Appending
  a stream title runs entirely inside OpenSearch's own script engine
  (append, cap at 10, dedupe tags) rather than search-service fetching
  the document, mutating it in Go, and writing it back -- avoids a
  lost-update race if two stream-created events for the same channel
  arrive close together.
- Search weights fields (`name^3`, `creator_name^2`, `stream_titles^2`)
  so a name match ranks above an incidental tag match, rather than
  treating every field as equally relevant.
- OpenSearch runs single-node with its security plugin disabled --
  local-dev posture, justified by there being nothing sensitive in the
  index (channel name/category/creator name/stream titles are all
  already public), not a stance that would hold for a real deployment.

## A real bug, and why it wasn't a bug in the usual sense

The first end-to-end run indexed nothing: `channel-created` produced
correctly (confirmed via `kafka-console-consumer` reading the topic
directly), but the `search-service-channels` consumer group never
appeared to process it -- no error logged, nothing. Added explicit
logging at both the fetch level and the handler level to stop guessing;
that isolated it to `FetchMessage` itself never returning a message for
that specific topic+group, even though the *identical* code pattern
(`stream-events` / `search-service-streams`) was consuming fine in the
same process.

The actual cause: repeated rapid container restarts during earlier
manual debugging had left the `search-service-channels` consumer group
in a stuck state in Kafka's own group metadata -- not a code defect, an
artifact of the debugging process itself hammering `docker compose up
--build` in quick succession without letting the previous consumer leave
the group cleanly. Confirmed by temporarily pointing the same code at a
fresh group ID (`search-service-channels-v2`), which consumed the
already-published message within 3 seconds. Fixed for real by deleting
the stuck group (`kafka-consumer-groups.sh --delete`) and reverting to
the original group name -- the code was correct throughout; documented
here because "the exact same consumer code, on the exact same topic
type, doesn't consume, with zero errors" is a genuinely confusing failure
mode worth having a name for next time it shows up.

## Verification

Full chain tested against the running stack: create a channel with a
distinctive category → `GET /search?q=<category>` finds it; a scheduled
stream's title becomes searchable via `AppendStreamTitle`; searching the
*creator's* display name finds their channel; searching a *tag* finds
it; a query missing `q` returns `400` rather than an empty result set
that could be mistaken for "no matches." Frontend: a debounced search box
on the home page, driven through a headless-browser pass confirming a
real result (with resolved creator name and most recent stream title)
renders from an actual keystroke, not a mocked response.
