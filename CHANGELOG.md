# Changelog

Los cambios relevantes de MonolithDTL se documentan en este archivo.

## [1.0.0] - 2026-10-17

### Añadido

- motor determinista de capital, liquidez, concentración HHI y stress ladder;
- escenario operativo `capital-stress`;
- tests unitarios Go y cobertura de integración TypeScript;
- documentación de arquitectura, modelo económico, integración y operación;
- política de seguridad y guía de contribución;
- validación cruzada en Windows y Linux;
- análisis de superficie pública e integridad de release.

### Cambiado

- contrato JSON identificado por `protocol`;
- métricas de reconciliación y reservas con terminología operativa;
- build y validación unificados en `npm run ci`;
- identidad visual común de Solguard Labs.

### Compatibilidad

- Go `1.22.x`;
- Node.js `24.x`;
- formato de estado `1.0.0`.
