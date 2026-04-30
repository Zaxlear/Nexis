# Nexis Console Web

This is the static Nexis Console frontend.

It intentionally uses no build step for the MVP. Nexis Console serves this directory from `apps/web` on port `47131`.

## Run

```bash
go run ./apps/console
```

Open:

```text
http://127.0.0.1:47131
```

The frontend calls:

```text
http://127.0.0.1:47141/api/v1
```

