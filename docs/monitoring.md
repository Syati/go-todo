# HTTP エラーと DB 接続異常の監視

HTTP アクセスログを Vector で集計し、Prometheus で 5xx 率と応答時間を表示します。Blackbox Exporter は 15 秒ごとに既存の `/up` を確認し、Prometheus は異常を Alertmanager に渡します。

## 設定

1. アプリの標準出力を監視ホストから読めるディレクトリの `http.jsonl` に転送します。[systemd で稼働させる構成](deployment.md) では、ユニットの `[Service]` に次を設定できます。同名ファイルが作られてから監視を起動し、標準出力の設定に合わせてログをローテーションします。

   ```ini
   LogsDirectory=go-todo
   StandardOutput=append:/var/log/go-todo/http.jsonl
   ```

2. [プローブ先の例](../ops/monitoring/probe-targets.example.yml) をコピーし、監視コンテナから接続できるアプリの HTTPS `/up` URL に書き換えます。
3. 当番通知サービスで Alertmanager の汎用 Webhook を受けられる URL を発行し、その URL だけをファイルに保存します。ホスト側で読み取りを制限した上で、コンテナ内の Alertmanager 実行ユーザーに読める権限を付けます。

```sh
export GO_TODO_LOG_DIR=/var/log/go-todo
export GO_TODO_PROBE_TARGETS=/etc/go-todo-monitor/probe-targets.yml
export GO_TODO_ALERT_URL_FILE=/etc/go-todo-monitor/alert-url
docker compose -f ops/monitoring/compose.yaml up -d
```

Prometheus は監視ホストの `127.0.0.1:19090` で開けます。時刻範囲を選び、次の式をグラフ表示します。

| 確認項目 | PromQL |
| --- | --- |
| 過去 5 分の HTTP 5xx 率（%） | `100 * go_todo:http_5xx_ratio:rate5m` |
| 過去 5 分の 95 パーセンタイル応答時間（秒） | `go_todo:http_duration_p95_seconds:rate5m` |
| DB 接続を含む稼働状態 | `probe_success{job="go-todo-up"}` |

Vector は監視開始後のログを集計し、読み取り位置を保存します。HTTP 指標は `/up` のプローブを除外します。`/up` は個別に監視し、HTTP 指標はアプリにアクセスがある期間に評価します。

## 通知を確認する

既定では 5 分間の 5xx 率が 5% を超えた状態、または `/up` の HTTP 200 以外が 30 秒続くと、当番用 Webhook に通知されます。しきい値と継続時間は [Prometheus のルール](../ops/monitoring/rules.yml) で変更できます。

本番前に検証環境で通常アクセスを流し、HTTP 指標と `probe_success` を表示します。故障を演習するときは検証用 DB を停止し、`/up` の 503 による通知を受信します。DB にアクセスする管理画面の操作も実行し、5xx 率の値と別の通知を確認します。DB を再開し、復旧後に `/up` が 200 を返すことを確認します。
