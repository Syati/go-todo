# PostgreSQL のバックアップと復元

UTC で毎日 02:00 と、DB マイグレーションの直前にバックアップを取得します。日次バックアップは 14 日間保持し、マイグレーション直前の分は切り戻し期間が終わるまで確保します。暗号化されたバックアップ先を使い、DB ホストの障害時にも取り出せるよう別のストレージへ複製します。

## バックアップ

PostgreSQL 18 のクライアントで実行します。接続先は libpq のサービス `go_todo`、認証情報はパーミッション 0600 のパスワードファイルなどに設定します。下記の処理を専用の運用アカウントの定期ジョブとして登録し、終了コードが 0 以外なら運用担当者に通知するよう設定します。

```sh
set -eu
umask 077
backup_dir=/var/backups/go-todo
mkdir -p "$backup_dir"
timestamp=$(date -u +%Y%m%dT%H%M%SZ)
archive="$backup_dir/go-todo-$timestamp.dump"
temporary=$(mktemp "$backup_dir/.dump.XXXXXX")
trap 'rm -f "$temporary"' EXIT

pg_dump --dbname=service=go_todo --format=custom --file="$temporary"
pg_restore --list "$temporary" >/dev/null
mv "$temporary" "$archive"
```

配布バージョンと `db status` をアーカイブの記録に添えます。複製を確認してから 14 日を過ぎた日次バックアップを削除します。マイグレーション前のアーカイブは日次削除の対象外の場所に置きます。

## 別の PostgreSQL に復元する

1. PostgreSQL 18 の隔離された復元先に空の DB とアプリ用ロールを用意します。復元の接続先を libpq サービス `go_todo_restore` に設定し、接続先のホストと DB 名を確認します。
2. アプリ用ロールで次を実行します。エラーがあれば切り替えを止め、復元先の状態を確認します。

   ```sh
   pg_restore --dbname=service=go_todo_restore --exit-on-error \
     --single-transaction --no-owner --no-privileges "$archive"
   ```

3. バックアップの記録にある版のアプリを復元先へ接続し、`db status` と `/up` を確認します。HTTPS でログインし、商品詳細で復元対象の SKU、価格、在庫を照合します。
4. 本番を復元先に切り替える場合は切り替え前に `psql --dbname=service=go_todo_restore --set=ON_ERROR_STOP=1 --command='TRUNCATE sessions;'` を実行します。バックアップ後に失効したセッションが復活するのを防げます。書き込み停止中に本番の `DATABASE_URL` を変更し、再起動後に `/up`、ログイン、商品と SKU の参照を再確認します。

演習では直近のアーカイブとマイグレーション直前のアーカイブを別々の空 DB に戻し、それぞれに対応する版のアプリで確認します。バックアップ時点、復元にかかった時間、確認した商品と SKU を運用記録に残します。
