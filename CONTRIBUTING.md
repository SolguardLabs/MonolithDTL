# Contribución

## Flujo De Trabajo

1. Cree una rama corta desde `main`.
2. Mantenga cada cambio limitado a una responsabilidad.
3. Añada tests Go para cálculo puro y tests TypeScript para contratos de CLI.
4. Ejecute `npm run ci` antes de abrir el pull request.
5. Explique el impacto contable, operativo y de compatibilidad.

## Convenciones

- Documentación y mensajes de usuario en español.
- Código, identificadores y comentarios de código en inglés.
- Importes como enteros en unidades mínimas; no use coma flotante.
- Colecciones ordenadas antes de calcular digests o serializar contratos.
- Errores con contexto, sin incluir secretos ni datos personales.
- Nuevos campos JSON con nombres estables en `snake_case`.

## Cambios Financieros

Un cambio en balances, deuda, shares, límites, capital o reconciliación debe
incluir:

- fórmula y unidades;
- ejemplo numérico con redondeos;
- condición de fallo;
- propiedad de conservación o invariante;
- test de borde y test de regresión;
- efecto sobre el `state_digest`.

## Pull Request

La descripción debe indicar:

- qué contrato cambia;
- qué archivos contienen la lógica;
- cómo se verificó;
- si existe migración o incompatibilidad;
- qué documentación se actualizó.

No incluya artefactos de `build/`, credenciales, snapshots con datos sensibles
ni resultados locales no reproducibles.
