# Task 5 — commerce-service (virtual gifting)

Implements the spec's **Viewer → Buy Coins → Send Gift → Creator** flow,
plus wallet/creator-balance/transaction-history. Same shape as Task 4:
own Postgres, charges via `payment-service` (the subscriber's/gifter's
own token forwarded, never asserted), channel/creator lookups via
`stream-service`.

| Method | Path | Auth | Notes |
|---|---|---|---|
| GET | `/healthz` | — | |
| GET | `/wallet` | bearer JWT | caller's own spendable coin balance |
| POST | `/wallet/buy-coins` | bearer JWT | `{coins}` — charges `coins * CentsPerCoin` (1 coin = 1 cent) via payment-service |
| POST | `/channels/{slug}/gift` | bearer JWT | `{gift_type}` — one of the fixed catalog (`internal/catalog`): rose(10)/heart(50)/diamond(500)/rocket(1000) |
| GET | `/creator/balance` | bearer JWT | caller's own earned-coins balance |
| GET | `/gifts/received` | bearer JWT | caller's own gift history as a recipient |
| GET | `/gifts/sent` | bearer JWT | caller's own gift history as a sender |

## Design notes

- **Two separate pools, not one balance.** `wallets.coin_balance`
  (bought with real money, spendable) and `creator_balances.earned_coins`
  (accumulated from gifts received) are different tables on purpose --
  they're not the same currency even though both are "coins". A creator's
  earned balance would need an actual payout/cash-out flow to become real
  money; that's out of scope here, same reasoning as recordings' public-
  bucket-vs-CloudFront trade-off (build the mechanism, flag what
  production would need on top).
- **`SendGift` is one Postgres transaction**, unlike the cross-service
  subscribe flow: debit sender, credit creator, insert the gift record
  all together, with `SELECT ... FOR UPDATE` on the sender's wallet row
  so two concurrent gifts from the same near-empty wallet resolve
  correctly instead of both reading a stale balance and both succeeding.
- Self-gifting and gifting with an unrecognized `gift_type` are both
  rejected before touching the ledger.
- `gift-events` on Kafka (`{type:"gift", gift_id, sender_id,
  recipient_id, channel_id, gift_type, coin_cost}`) is Task 6 fuel, same
  as `subscription-events` -- produced, not yet consumed by anything.

## Verification

Buy coins → wallet credited; send a gift → sender debited, creator's
earned balance credited by the same amount, gift appears in both
`/gifts/received` and `/gifts/sent`; insufficient balance rejected
(`402`) without touching either balance; unknown `gift_type` rejected
(`400`) before any DB write; self-gift rejected (`400`); the payment
decline sentinel applied to a coin purchase correctly declines without
crediting the wallet. Frontend: buy-coins and send-gift driven through a
headless-browser pass against the real UI, confirming the wallet balance
actually reaches `0` after spending it -- not just that the request
returned `201`.
