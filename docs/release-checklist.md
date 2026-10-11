# Release-Checkliste v1.0.0 und danach

Der Weg vom letzten Beta zum ersten fertigen Release
([#18](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/18)),
und was danach für winget und den Microsoft Store kommt
([#131](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/131)).
Tags und Releases macht nur der Owner.

## Vor `v1.0.0-rc.1`

- [ ] Offsets für die aktuelle Among-Us-Version eingebaut und live geprüft (#198)
- [ ] Alle Punkte in [live-tests.md](live-tests.md) abgehakt: #141, #142, #144, #136
- [ ] #157 durch, #160 entschieden (behoben oder als bekannte Einschränkung)
- [ ] `python scripts/local_check.py` grün, alle 17 Schritte
- [ ] `CHANGELOG.md`: Abschnitt `Unreleased` wird `v1.0.0-rc.1 — Datum`
- [ ] Anleitungen nennen keine Beta-Einschränkungen mehr, die nicht mehr gelten

## `v1.0.0-rc.1`

- [ ] `python scripts/local_check.py --release --version v1.0.0-rc.1`
- [ ] Mit `scripts/local_release.py` bauen und veröffentlichen (Vorabversion)
- [ ] In einer echten Runde prüfen, Update von `v0.1.7-beta` aus

## `v1.0.0`

- [ ] Tag `v1.0.0` auf denselben Stand wie `rc.1`, falls nichts mehr geändert wurde
- [ ] `release.yml` baut aus dem Tag: Setup, Zip, `SHA256SUMS`, Release-Notes
- [ ] Release-Notes und `SHA256SUMS` gegenlesen
- [ ] README: Abschnitt *Status* auf fertig umstellen
- [ ] #18 und #21 schließen

## Nach `v1.0.0`: winget (#131)

Entwürfe der Manifeste liegen in [`installer/winget/`](../installer/winget/).
Sie gelten nur für stabile Releases.

1. `PackageVersion`, `InstallerUrl` und `InstallerSha256` aus dem Release
   eintragen; die Prüfsumme steht in `SHA256SUMS`.
2. Lokal prüfen:
   `winget validate --manifest installer\winget` und
   `winget install --manifest installer\winget`.
3. Erste Einreichung in `microsoft/winget-pkgs` unter dem Konto des Owners,
   erst nach seiner Freigabe. Ordner dort: `manifests/a/AUVC/Capture/1.0.0/`.
4. Folgeversionen mit `wingetcreate update AUVC.Capture --version … --urls …`,
   weiterhin nur nach Freigabe.
5. Anleitung ergänzen: `winget install AUVC.Capture`.

## Nach `v1.0.0`: Microsoft Store (#131)

Nur Machbarkeit, keine Einreichung ohne Entscheidung des Owners.

- [ ] MSIX mit `runFullTrust` bauen, lokal mit Testzertifikat installieren
- [ ] Darin prüfen: Among Us lesen, Bot starten samt Port, Token (DPAPI),
      Kopplung, Deinstallation
- [ ] Richtlinien klären: `runFullTrust`, Among-Us-Bilder, Lesen des Spielspeichers
- [ ] Entscheidung des Owners zur Registrierung als Einzelentwickler
