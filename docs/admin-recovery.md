# 特権管理者の復旧

有効な特権管理者が1人だけで本人がログインできない場合に使います。承認を受けた運用担当者が、対象環境を確認し、その環境のアプリ用 DB へ接続できる運用ホストで実行します。実行者の運用アカウントと対応チケットを記録しておきます。

その環境と同じバージョンのバイナリを用意し、対象の `DATABASE_URL` を通常のシークレット管理から注入します。以下は Bash の例です。`alice / INC-123` は実行者と承認記録を照合できる識別子に置き換えます。

```bash
(
  read -r -s -p '新しいパスワード（12〜72バイト）: ' RECOVERY_PASSWORD
  printf '\n'
  export RECOVERY_PASSWORD
  ./go-todo --env production admin recover admin@example.test 'alice / INC-123'
)
```

コマンドは指定メールが唯一の有効な特権管理者の場合に、パスワード更新、既存セッションの失効、ログイン試行制限の解除、復旧記録の追加を同じトランザクションで行います。パスワードは承認された経路で本人に渡し、新しいパスワードでログインできることを確認します。

監査時に DB の参照権限を持つ担当者は、記録を次のように確認できます。

```sql
SELECT occurred_at, admin_id, operator_ref
FROM admin_recovery_events
ORDER BY occurred_at DESC, id DESC
LIMIT 20;
```
