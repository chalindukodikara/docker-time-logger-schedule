# Project: **OpenChoreo – Build System Redesign**

OpenChoreo is an **Internal Developer Platform (IDP)** that lets teams ship **Services, Web Apps, API Proxies, Scheduled & Manual Tasks, Event Handlers, and Test Runners** from _creation → build → deploy → observe_.  
Everything is implemented Kubernetes-style: CRDs + controllers (Kubebuilder), with one **Control Plane** and many **Data / Build Planes**.

We are **redesigning the Build flow** and its CRDs to make builds:

* agnostic to Argo Workflows _or_ Tekton Pipelines  
* templatable and reusable across component types  
* clearer to reason about in code and YAML  
* future-proof for multi-cluster & GitOps

Gemini CLI will act as our code-generation & scaffolding co-pilot.  
These are the guard-rails it must follow.

## 2. Target CRDs (v1alpha1)

| Kind | Purpose | Notes |
|------|---------|-------|
| **Build** | Represents a single build execution (immutable after creation). | `spec` references source & parameters; `status` exposes conditions and artifact refs. |
| **BuildClass** | A higher-level preset that explains *how* to build (engine + params). | Users pick one; platform operators own the spec. |
| **BuildTemplate** | Low-level template that expands into engine YAML (Argo / Tekton). | Referenced by `BuildClass`; validated by admission webhook. |
| **Workflow** | Argo or Tekton workflow. | Internally create this using above 3 |

> **Design goal:** Design CRDs to divide the work for PEs and Developers so that this works effectively as an IDP.

---



## 4. YAML / Kubebuilder Annotations

* CRD structs use `controller-gen`; annotate fields (`+kubebuilder:validation:`) accurately.  
* Defaulting / validating webhook code lives in `apis/build/v1alpha1/webhook.go`.  
* Example manifests in `config/samples/` must apply cleanly with `kubectl apply`.

---

## 5. Compatibility Requirements

1. **Engines:** Argo ≥ v3.5, Tekton ≥ v0.55.  
2. **Kubernetes:** 1.26 – 1.29.  
3. **Multi-cluster:** A build may run in any cluster that has label `core.choreo.dev/cluster-role=build-plane`.  
4. **GitOps:** Reconciler must be idempotent; never mutate `spec` after create.

---

## 6. Gemini-Specific Directives

* **CRD Go types:** include `+kubebuilder:resource:path=builds,scope=Namespaced`, etc.  
* **Controller code:** scaffold with Kubebuilder patterns, but **merge** into existing files—do not overwrite.  
* **Dependencies:** add none unless absolutely required; justify in the PR.  
* Prefer composition; avoid huge `switch` blocks.  
* **Tests:** every new public method needs Ginkgo & Gomega unit tests.

---

## 7. Example Tasks for Gemini

1. **Add field** `timeoutSeconds` to `Build.spec`, default = 3600, max = 86400.  
2. **Generate admission webhook** that forbids specifying both `spec.buildPack` and `spec.dockerfile`.  
3. **Scaffold controller logic** to map `Build.spec.parameters` into Argo Workflow parameters.

---

### TL;DR for Gemini

> You are extending OpenChoreo’s Build subsystem (Go 1.22, Kubebuilder).  
> Produce idiomatic Go CRDs, admission webhooks, and controllers that obey the rules above and fit seamlessly into the existing repo.
