# Install

## From source

```bash
git clone https://github.com/sajonaro/mgtt.git
cd mgtt
go build ./cmd/mgtt
sudo mv mgtt /usr/local/bin/
```

## Docker

```bash
docker compose run --rm mgtt version
```

## Install providers

```bash
mgtt provider install kubernetes aws
```
