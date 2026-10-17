# Operación Y Runbooks

## Estados Operativos

MonolithDTL separa el resultado funcional de una operación del estado de riesgo
de la cartera. Una ejecución puede completar sus transiciones y producir un
assessment `restricted` si la holgura de capital o liquidez cae bajo el umbral.

```mermaid
stateDiagram-v2
    [*] --> open
    open --> restricted: ratio preventivo o liquidez baja
    restricted --> open: exposición reducida o capital añadido
    restricted --> halted: ratio bajo umbral de detención
    halted --> restricted: remediación parcial validada
    halted --> open: recuperación completa y revisión
```

| Estado       | Nuevas operaciones             | Acción requerida                      |
| ------------ | ------------------------------ | ------------------------------------- |
| `open`       | normales dentro de límites     | monitorización                        |
| `restricted` | limitar exposición incremental | revisar drivers y aprobar excepciones |
| `halted`     | detener admisiones             | preservar evidencia y remediar        |

## Arranque Verificado

```bash
git status --short
npm ci
npm run ci
npm run build
build/monolithdtl scenario baseline > baseline.json
```

Compruebe:

- estado Git limpio;
- versión `1.0.0` en package y lockfile;
- CI local completa;
- `state_digest` válido;
- invariantes verdaderas para el escenario base;
- capital y liquidez por encima de los umbrales.

## Monitorización

Métricas mínimas:

| Métrica                        | Interpretación              | Umbral inicial         |
| ------------------------------ | --------------------------- | ---------------------- |
| `risk.local_debt`              | obligaciones abiertas       | tendencia por producto |
| `risk.global_debt`             | deuda agregada              | comparar con local     |
| `risk.reconciliation_variance` | diferencia de conciliación  | `0`                    |
| `risk.reserve_variance`        | varianza en reservas        | `0`                    |
| `capital.required`             | capital calculado           | bajo available         |
| `capital.available`            | seguro + tesorería          | por encima del mínimo  |
| `reconcile.drift`              | detalle por cuenta y activo | `0`                    |

No alerte únicamente por volumen. Una subida de exposición puede ser válida si
capital, liquidez, límites e invariantes mantienen holgura.

## Runbook De Reconciliación

```mermaid
flowchart TD
    ALERT[Varianza detectada] --> FREEZE[Restringir producto o cuenta]
    FREEZE --> SNAP[Guardar JSON, commit y binario]
    SNAP --> TRACE[Seguir refs y ledger entries]
    TRACE --> LOCAL[Comparar obligación local]
    TRACE --> GLOBAL[Comparar deuda agregada]
    LOCAL --> CAUSE[Identificar transición causal]
    GLOBAL --> CAUSE
    CAUSE --> FIX[Preparar corrección y test]
    FIX --> REVIEW[Revisión independiente]
    REVIEW --> VALIDATE[CI Windows + Linux]
    VALIDATE --> RESUME[Reabrir de forma gradual]
```

Nunca modifique directamente un mapa para hacer desaparecer una varianza. Use
una transición de negocio o una entrada compensatoria explícita, documentada y
aprobada.

## Runbook De Capital

Para `restricted`:

1. identifique la mayor `ExposureCharge`;
2. revise HHI y clusters dominantes;
3. compare base, adverse, severe y reverse;
4. reduzca admisiones del driver dominante;
5. evalúe capital adicional o vencimiento de exposición;
6. repita el assessment con los mismos parámetros.

Para `halted`:

1. detenga nuevas exposiciones;
2. conserve el snapshot que originó la decisión;
3. verifique que no exista error de unidad o parámetro;
4. establezca un plan de reducción o recapitalización;
5. exija dos revisiones antes de reabrir;
6. pase primero por `restricted` salvo recuperación completa demostrable.

## Rotación De Operador

```mermaid
sequenceDiagram
    participant Admin
    participant Registry as OperatorRegistry
    participant Monitor
    participant Product

    Admin->>Registry: deactivate(old, reason)
    Registry-->>Monitor: operator_deactivated
    Admin->>Registry: register(new, limit, products)
    Admin->>Registry: authorize(new, product caps)
    Monitor->>Registry: verificar utilización = 0
    Registry->>Product: habilitar flujo gradual
```

Antes de reemplazar un operador, confirme que no existan batches creados que
dependan de su identidad. La utilización histórica permanece en los eventos.

## Recuperación De Un Escenario Fallido

La CLI construye el contexto en memoria, por lo que un fallo no deja estado
persistente. Para reproducir:

```bash
build/monolithdtl scenario <nombre> > observed.json
sha256sum build/monolithdtl
git rev-parse HEAD
```

En PowerShell:

```powershell
build\monolithdtl.exe scenario <nombre> | Set-Content observed.json
Get-FileHash build\monolithdtl.exe -Algorithm SHA256
git rev-parse HEAD
```

Conserve salida, error, hash de binario, commit, SO y arquitectura.

## Release

Secuencia mínima:

1. CI verde en la rama `release/**`;
2. pull request revisado y fusionado;
3. CI verde sobre `main`;
4. rama `production` apuntando al mismo commit;
5. tag anotado `vX.Y.Z` sobre ese commit;
6. CI verde del tag;
7. release de GitHub publicada;
8. control de integridad del evento `release` verde.

```mermaid
flowchart LR
    FEATURE[release/**] --> PR[Pull request]
    PR --> MAIN[main]
    MAIN --> PROD[production]
    PROD --> TAG[vX.Y.Z]
    TAG --> RELEASE[GitHub release]
    RELEASE --> VERIFY[Integridad final]
```

No anuncie la entrega mientras los cuatro refs o estados no converjan en el
mismo commit y sus checks no hayan terminado.
