# Task 8 — recommendation-service

A personalized "For You" feed, ranked by category affinity built from a
viewer's own behavior (views, subscriptions, gifts). No ML model --
weighted Redis sorted sets, explicitly the baseline a real recommender
would sit on top of, not a placeholder for something smarter that got
cut.

## Endpoint

| Method | Path | Auth | Notes |
|---|---|---|---|
| GET | `/healthz` | — | |
| GET | `/feed` | required | all channels (via `stream-service`), re-ranked by the caller's affinity scores; 60s Redis cache per user |

`stream-service` also gained one endpoint this task, since a feed needs
view signal to rank on:

| Method | Path | Auth | Notes |
|---|---|---|---|
| POST | `/channels/{slug}/view` | required | fires a `view` event on `user-events`; `204` with no body |

## The feature pipeline

`recommendation-service` writes nothing of its own except a Redis sorted
set per user (`affinity:<user_id>`, member = category, score = weighted
count). It's a pure reactor to three Kafka topics, each already produced
by an existing service:

| Kafka topic | Event | Weight | Category resolution |
|---|---|---|---|
| `user-events` (new) | `view` | 1.0 | already on the payload (`stream-service` looks it up once when recording the view) |
| `subscription-events` | `subscribed` | 5.0 | resolved via `GET /internal/channels/{id}` on `stream-service` |
| `gift-events` | `gift` | 10.0 | resolved via the same internal lookup |

Weights are ordinal, not calibrated -- subscribing and gifting are
stronger signals of genuine interest than a view, so they count for
more, in the same relative order a hand-tuned scorer would use.

`GET /feed` pulls the caller's top categories (`ZREVRANGE ... WITHSCORES`),
pulls the full channel list from `stream-service`, and sorts channels by
their category's score (0 for any category the viewer has no affinity
for yet -- a cold-start viewer just gets `stream-service`'s default
order, unchanged). The result is cached in Redis for 60 seconds per user
so a page of repeat feed requests doesn't hit `stream-service` and
re-sort on every load; the cache is a pure read-through and holds no
signal itself, so it can't go stale in a way that hides new activity for
longer than the TTL.

## Design notes

- **Redis sorted sets, not a table.** `ZINCRBY` for the write path and
  `ZREVRANGE WITHSCORES` for the read path give an atomic increment and
  a pre-sorted top-N in one command each -- no read-modify-write race
  between two events for the same user landing close together, and no
  need for a separate ranking step in application code.
- **Category, not per-channel, is the unit of affinity.** A viewer who
  watches three different gaming channels should see *other* gaming
  channels surface, not just the same three -- scoring at the category
  level is what makes the feed generalize instead of just echoing
  history back.
- **Three consumer groups, one per topic, from the start.** Given
  `subscription-events` and `gift-events` are consumed by other services
  too, and Task 6/7 both hit silent-failure bugs from sharing one group
  ID across topics, this service was built with distinct group IDs
  (`recommendation-service-views` / `-subscriptions` / `-gifts`) up
  front rather than as a fix.

## The same stuck-consumer-group failure mode, a third time

Verification hit the identical symptom documented in
[task-7-search.md](task-7-search.md): view events landed on `user-events`
(confirmed directly via `kafka-console-consumer.sh --from-beginning`),
`recommendation-service` logged no errors, and the affinity set in Redis
stayed empty. Same root cause -- rapid `docker compose up --build`
cycles during manual debugging left `recommendation-service-views` (and,
this time, `-subscriptions` and `-gifts` too, since all three consumers
had been restarted together) stuck in Kafka's group metadata. Same fix:
stop the container, wait out the ~15s session timeout, `delete` each
group, restart with the original group names.

This is now the third time this exact failure has shown up (Task 6,
Task 7, Task 8), always from the same cause -- fast local
restart/rebuild loops outrunning Kafka's consumer group cleanup, not a
code defect. It's a diagnostic playbook at this point rather than a
one-off bug: check the topic directly to rule out a producer problem,
add temporary fetch/handler-level logging to confirm the consumer
goroutine is even running, test with a throwaway group-ID suffix to
confirm the code is correct, and if that works, delete-and-restart the
real group instead of continuing to debug code that isn't broken.

## Verification

Ran against the live stack with a fresh viewer account to get an
unambiguous baseline:

1. `GET /feed` before any activity returns all channels at score 0, in
   `stream-service`'s default order -- correct cold-start behavior.
2. 5x `POST /channels/{gaming-slug}/view` → `affinity:<id>` shows
   `gaming = 5` (5 x `WeightView`).
3. Subscribing to a cooking-category channel → `cooking = 5` appears
   (1 x `WeightSubscribed`), `gaming` unchanged.
4. Sending a gift to the gaming channel → `gaming` jumps to `15`
   (5 x `WeightView` + 1 x `WeightGift`), confirming weights accumulate
   correctly across event types on the same category.
5. `GET /feed` after the 60s cache from step 1 expired: both gaming
   channels rank at the top (score 15), cooking channel next (score 5),
   everything else at 0 in original order -- confirms the feed actually
   re-ranks on real signal rather than just accepting/storing it.

An earlier test pass (before the consumer-group fix above) showed an
affinity score of 6 after only 3 view events -- traced to events from a
prior, interrupted debugging attempt still sitting unconsumed on the
topic and being replayed once the stuck group was deleted and consumption
resumed from the earliest offset. Not a double-counting bug: the clean
re-run in step 2 above (5 views -> exactly 5) confirms the pipeline is
1:1 between events and score.
