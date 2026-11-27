# デプロイと切り戻し

Web サーバーと goose のマイグレーションは同じバイナリに入っています。以下は Linux、systemd、PostgreSQL 18、HTTPS のリバースプロキシを使う例です。

## 配布物を作る

配布先と同じ OS・アーキテクチャのビルド環境で、対象の Git コミットをクリーンな作業ツリーにチェックアウトして実行します。

```sh
npm ci --prefix assets
make build
./bin/go-todo --version
sha256sum bin/go-todo
```

`--version` が表示した Git リビジョン、ハッシュ、対象 OS・アーキテクチャを配布物と一緒に保存します。サーバー起動時の JSON ログにも同じバージョンが出ます。

## サーバーを設定する

アプリは専用の OS ユーザー `go-todo` で実行します。リビジョンごとのバイナリを `/opt/go-todo/releases/<revision>/go-todo` に置き、`/opt/go-todo/current` は稼働中のリビジョンを指すシンボリックリンクにします。プロキシでは HTTPS を終端し、アプリのループバックアドレスへ転送します。

`/etc/go-todo/app.env` に `ADDR=127.0.0.1:8080` と、シークレット管理から取得した `DATABASE_URL` を設定します。このファイルは root 所有・モード 0600 とし、DB 接続には運用環境に合った TLS 検証設定を使用します。`--env production` はセッション Cookie の Secure 属性を有効にします。

```ini
[Unit]
Description=Go Todo
After=network-online.target
Wants=network-online.target

[Service]
User=go-todo
EnvironmentFile=/etc/go-todo/app.env
ExecStart=/opt/go-todo/current/go-todo --env production serve
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

ユニットを `/etc/systemd/system/go-todo.service` に置き、`systemctl daemon-reload` を実行します。

## リリースする

1. 前回のリビジョン、起動時のバージョン、DB の `db status` を記録します。SQL の変更があれば、旧バイナリとの互換性と、復元できるバックアップを確認します。非互換の変更は書き込み停止中に行います。
2. 新バイナリのハッシュと `--version` を照合します。DB のシークレットを注入した環境で、新バイナリの `--env production db up` を実行し、`db status` で適用結果を確認します。初回だけ、空の DB で `admin bootstrap` を実行します。
3. `systemctl stop go-todo` で旧プロセスを停止し、`current` を新リビジョンへ切り替えて `systemctl start go-todo` を実行します。初回は停止を省略できます。
4. `journalctl -u go-todo -n 30` でバージョンを確認します。HTTPS 経由の `/up` が 200 を返し、管理者でログインして商品一覧が開くことを確認します。

## 切り戻す

- 旧バイナリが適用済みスキーマで動く場合は、サービスを停止して `current` を前回のリビジョンに戻します。起動後に同じ確認を行います。
- 非互換の場合は、書き込みの停止を維持して変更後の DB も退避します。マイグレーションの `Down` がデータを保持することを検証済みなら、新バイナリで必要なステップだけ `db down` を実行します。列やテーブルの削除などでデータが失われる場合は、変更前のバックアップを別 DB に復元し、リリース後に受け付けた更新の扱いを決めてから接続先を切り替えます。旧バイナリを起動し、`/up`、ログイン、商品・SKU の参照を確認します。

本番に適用する前に、同じ PostgreSQL メジャーバージョンと HTTPS 設定で、リリースと必要な切り戻し経路を演習します。確認したリビジョン、DB バージョン、復元したバックアップの時点をリリース記録に残します。
