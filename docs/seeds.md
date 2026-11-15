# seed の使い方

`rails8-todo/db/seeds.rb` と同じく、環境ごとの入口からデータを依存順に登録します。
データは YAML に記述し、Go は読み込み・検証・登録だけを担当します。
開発環境では初期管理者の環境変数を入力せず、次のコマンドで画面を試すデータを準備できます。

```sh
rtk make up
rtk make migrate
rtk make seed
rtk make run
```

開発用のログイン情報は Rails 版と同じです。

| メールアドレス | パスワード | 権限・状態 |
| --- | --- | --- |
| `admin@example.com` | `password123` | 特権管理者 |
| `operator01@example.com` 〜 `operator40@example.com` | `password123` | オペレーター |
| `deleted@example.com` | `password123` | 論理削除済み。ログイン不可 |

この固定パスワードは開発用 fixture だけに使います。
通常の管理者登録・パスワード変更と `admin bootstrap` は、引き続き12〜72バイトを要求します。

## 登録される商品

Rails 版と同じ名称・説明・ステータス・税率・SKUコード・価格・在庫を使います。

| 商品 | ステータス | 税率 | SKU数 |
| --- | --- | --- | --- |
| オーガニックコットン T シャツ | 販売中 | 10% | 6 |
| ランニングシューズ | 販売中 | 10% | 3 |
| 有機緑茶 100g | 販売中 | 8% | 1 |
| コーヒー豆 ブレンド | 販売中 | 8% | 2 |
| マグカップ | 下書き | 10% | 1 |
| キャンバストート（旧型） | 販売終了 | 10% | 0 |

管理者は論理削除済みを含めて42件、商品は6件、SKUは13件です。
管理者は1ページ30件を超えるので、ページ切り替えも確認できます。
Rails 版のロック済み管理者と OAuth クライアント・トークンは、Go 側の機能が未実装のため登録しません。

## 再実行時の動作

- 管理者はメールアドレス、商品は名称、SKUはコードで既存データを判定します。
- 未登録のレコードだけを追加します。パスワード・権限・説明・価格・在庫などを初期値へ戻しません。
- 論理削除済みの管理者も既存として扱い、復活させません。
- 全データを1つのトランザクションで登録し、途中で失敗したらその実行の変更を戻します。
- seed 同士の同時実行は PostgreSQL の advisory lock で直列化します。

既存データの編集結果を保つため、名称やコードを変更すると元のキーのレコードが再度追加される場合があります。
既存データとバリエーションの一意制約が衝突した場合などは、エラーを解消してから再実行してください。

## 環境の扱い

`development` にだけ seed を定義しています。
Rails 版と同じく、入口がない環境ではデータを変更せず、未定義であることを表示します。
本番の初期管理者は環境変数を指定して `admin bootstrap` で作成してください。
`BOOTSTRAP_EMAIL` と `BOOTSTRAP_PASSWORD` は bootstrap 専用で、開発用 seed のデータは変更しません。

```sh
rtk proxy ./bin/go-todo --env development db seed
```

seed の YAML は `go:embed` でバイナリに埋め込むため、配布済みバイナリから実行する場合も外部の seed ファイルは不要です。
定義を変更した場合は再ビルドしてください。migration は自動適用しないので、先に `db up` を実行します。

## 構成と追加手順

```text
db/seeds/
  seeds.go                     環境ごとの振り分け
  development/
    data/
      admins.yml               管理者データと共通の開発用パスワード
      products.yml             商品・SKUデータ
    data.go                    YAML の埋め込みと厳密な読み込み
    validation.go              データの検証・正規化
    development.go             トランザクションと実行順
    admins.go                  管理者の登録処理
    products.go                商品・SKUの登録処理
db/queries/seeds.sql            sqlc 用クエリ
internal/cli/seed.go            Cobra の db seed コマンド
```

既存の種類のデータ追加・修正は、`development/data/` の対応する YAML だけを編集します。例えば管理者は次の形式です。

```yaml
password: password123
admins:
  - email: admin@example.com
    role: super_admin
  - email: operator01@example.com
    role: operator
  - email: deleted@example.com
    role: operator
    deleted: true
```

商品は SKU を入れ子にします。SKU コードは並び順から生成せず、明示的に指定します。
`size`・`color`・`description` は省略すると空文字になります。

```yaml
products:
  - name: マグカップ
    description: 来月から販売予定。
    status: draft
    tax_rate: 10
    skus:
      - code: MUG-STD-01
        color: ベージュ
        price_yen: 2200
        stock_quantity: 0
```

読み込み時に未知のフィールド・不正な型・複数の YAML ドキュメントを拒否します。
メールアドレス・権限・商品・SKU の値と、データ内のメールアドレス・商品名・SKUコード・バリエーションの重複も検証します。
すべてのファイルを読み込み・検証してから DB トランザクションを開始します。

新しい種類の seed を追加する場合は、対応する YAML と読み込み・登録処理を作り、`development.go` などの入口で依存順に呼び出します。
クエリは `db/queries` に定義し、`rtk make generate-sql` で生成します。
schema の変更は seed ではなく goose migration に記述します。

## 検証

```sh
rtk proxy env 'TEST_DATABASE_URL=postgres://go_todo:local-development-only@127.0.0.1:5433/go_todo?sslmode=disable' make test
```

テストは専用スキーマを使い、初回登録・再実行時の件数と編集内容の保持・失敗時の rollback を確認します。
設定ファイルのないディレクトリから `db seed` を実行するケースも含みます。
YAML の読み込み・検証テストは DB なしで実行されるため、GitHub Actions の unit test にも含まれます。
