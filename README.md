# Go Kubernetes Controller PoC

Go と Kubernetes の仕組みを理解するために作成している学習用 PoC です。

`client-go` を使用して Kubernetes API にアクセスし、
Deployment の状態取得・Scale・Reconcile を実装しています。

現在は、一定間隔で Deployment の replicas を確認し、
実際の状態が Desired State と異なる場合に、
指定した replicas へ自動的に戻すところまで実装しています。

> このリポジトリは個人学習用の PoC です。
> 過去の業務コード・設計・データ等は使用していません。

---

## 目的

以前の業務で Go / Kubernetes を扱った経験をもとに、
Kubernetes Controller の基本的な考え方を
公開可能な最小構成で再構築することを目的としています。

単に Kubernetes API を呼び出すだけではなく、

```text
現在状態を取得
    ↓
Desired State と比較
    ↓
差分を検出
    ↓
Kubernetes API を操作
    ↓
Desired State へ収束
```

という Reconciliation の基本的な流れを、
実際にコードを書きながら確認しています。

---

## 現在実装している機能

- kubeconfig を使用した Kubernetes API への接続
- Deployment 一覧の取得
- Deployment の replicas 取得
- Deployment の Scale
- Current State / Desired State の比較
- 差分がある場合のみ Scale を実行
- 一定間隔で Reconcile を繰り返す Controller Loop

現在は **5秒間隔の Polling** 方式です。

Watch / Informer を利用したイベント駆動方式は、
現時点では未実装です。

---

## 構成

```text
go-k8s-controller-poc/
├── cmd/
│   └── controller/
│       └── main.go
├── internal/
│   └── kubernetes/
│       └── client.go
├── manifests/
│   ├── namespace.yaml
│   └── deployment.yaml
├── docs/
│   └── architecture.md
├── bin/
│   └── controller
├── .gitignore
├── Dockerfile
├── Makefile
├── README.md
└── go.mod
```

`bin/` はビルド成果物のため Git 管理対象外です。

---

## 使用技術

- Go
- Kubernetes
- client-go
- kubectl
- kind
- Docker
- WSL2

ローカル Kubernetes クラスタとして kind を使用しています。

---

# 動作概要

現在の PoC では、`sample-app` Deployment の
Desired State を以下のように設定しています。

```text
desired replicas = 3
```

Controller は一定間隔で現在の replicas を取得します。

現在値が Desired State と一致している場合は、
Kubernetes API の更新を行いません。

```text
current=3
desired=3

→ No scaling required
```

一致していない場合は Scale を実行します。

```text
current=1
desired=3

→ Scaling deployment 1 -> 3
```

---

# セットアップ

## Kubernetes クラスタの確認

```bash
kubectl get nodes
```

kind クラスタが起動していることを確認します。

例：

```text
NAME                       STATUS   ROLES           VERSION
go-k8s-poc-control-plane   Ready    control-plane   v1.37.0
```

---

## Namespace / Deployment の作成

```bash
kubectl apply -f manifests/namespace.yaml
kubectl apply -f manifests/deployment.yaml
```

確認：

```bash
kubectl get deployment -n go-k8s-poc
kubectl get pods -n go-k8s-poc
```

---

# Build

```bash
go build -o bin/controller ./cmd/controller
```

テスト：

```bash
go test ./...
```

現時点では Controller の単体テストは未実装です。

---

# Controller の起動

```bash
./bin/controller
```

起動すると Deployment の一覧を取得した後、
Reconcile Loop が開始します。

例：

```text
Found 3 deployment(s)
namespace=go-k8s-poc name=sample-app replicas=3
namespace=kube-system name=coredns replicas=2
namespace=local-path-storage name=local-path-provisioner replicas=1

Controller started

Reconciling deployment go-k8s-poc/sample-app: current=3 desired=3
No scaling required
```

以降、一定間隔で Reconcile を実行します。

---

# Reconcile の動作確認

Controller を起動した状態で、
別のターミナルから Deployment の replicas を変更します。

```bash
kubectl scale deployment sample-app \
  --replicas=1 \
  -n go-k8s-poc
```

Controller 側で差分を検出します。

```text
Reconciling deployment go-k8s-poc/sample-app: current=1 desired=3
Scaling deployment go-k8s-poc/sample-app: 1 -> 3
```

再度 Deployment を確認します。

```bash
kubectl get deployment sample-app -n go-k8s-poc
```

例：

```text
NAME         READY   UP-TO-DATE   AVAILABLE
sample-app   3/3     3            3
```

Pod についても確認できます。

```bash
kubectl get pods -n go-k8s-poc
```

この結果から、

```text
replicas = 3
    ↓
外部操作で replicas = 1
    ↓
Controller が差分を検出
    ↓
replicas = 3 へ Scale
    ↓
Desired State へ復旧
```

という一連の Reconciliation を確認できます。

---

# 現在の実装について

この PoC は Kubernetes Controller の基本概念を理解するため、
意図的にシンプルな構成にしています。

現時点では、

```text
5秒ごとに Kubernetes API を確認
```

する Polling 方式です。

そのため、本格的な Kubernetes Controller で利用される
Informer / Watch / Workqueue 等を利用した構成とは異なります。

まず、

```text
Observe
↓
Compare
↓
Reconcile
```

という基本動作を自分で実装し、
その後 Kubernetes 標準の Controller パターンへ発展させる方針です。

---

# 今後の予定

次の段階では、以下を順番に検証する予定です。

1. Reconcile 処理の単体テスト
   - Desired State と一致している場合
   - replicas が不足している場合
   - Deployment が存在しない場合

2. `client-go` fake client を利用したテスト

3. Context timeout / graceful shutdown の追加

4. Polling 方式から Watch / Informer 方式への変更

5. Controller の責務分離

6. controller-runtime の検証

7. 必要に応じて CRD / Custom Controller の検証

---

# 将来的な検証アイデア

別途作成している性能試験分析 PoC と組み合わせ、

```text
Performance Test
        ↓
Python
結果整理 / 閾値判定
        ↓
判定結果
        ↓
Go Controller
        ↓
Kubernetes API
        ↓
Scale / Control
```

のように、

**性能評価と Kubernetes 制御を分離した構成**

についても検証したいと考えています。

ただし、性能試験結果だけから自動的に Scale を決定するのではなく、
判定根拠・責任分界・安全な制御方法を含めて検討する予定です。

---

# 現在の到達点

現時点では、

```text
Go
 ↓
client-go
 ↓
Kubernetes API
 ↓
Deployment
 ↓
Reconcile
 ↓
Desired State への自動復旧
```

まで動作確認済みです。

次の目標は、
**「動く Controller」から「テスト可能で壊れにくい Controller」へ進めること**
です。