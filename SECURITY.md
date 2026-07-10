# Security Policy

MonolithDTL es un laboratorio vulnerable por diseno. No debe desplegarse en
produccion ni reutilizarse como motor financiero real.

## Alcance

El alcance del CTF incluye:

- reconciliacion entre obligaciones locales y deuda global;
- redenciones de vault;
- settlement de escrow;
- rutas multi-producto;
- limites de operadores;
- eventos de auditoria emitidos por la CLI.

Quedan fuera de alcance:

- ataques de red;
- explotacion del sistema operativo;
- denegacion de servicio por consumo de recursos;
- cambios en scripts de build que no afecten el modelo economico.

## Reportes

Los hallazgos deben explicar:

1. el flujo de producto que crea el estado inconsistente;
2. la diferencia entre deuda local y deuda global;
3. el impacto economico en reservas o redenciones;
4. una mitigacion que preserve trazabilidad por producto.

## Severidad Esperada

La vulnerabilidad principal es critica porque permite redenciones cruzadas,
deuda fantasma y robo indirecto de reservas sin violar controles locales de un
producto individual.

