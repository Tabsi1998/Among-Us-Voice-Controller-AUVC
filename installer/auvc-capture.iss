; AUVC capture installer.
;
; Built by the release workflow with:
;   iscc /DAppVersion=1.2.3 /DPayloadDir=..\artifacts\capture\win-x64 auvc-capture.iss
;
; See docs/installer-decision.md for why Inno Setup, what unsigned means for
; whoever downloads this, and what the uninstaller asks.

#ifndef AppVersion
  #define AppVersion "0.0.0-dev"
#endif
#ifndef PayloadDir
  #define PayloadDir "..\artifacts\capture\win-x64"
#endif
#ifndef OutputDir
  #define OutputDir "..\artifacts\release"
#endif

; VersionInfoVersion is the Windows file-version resource and takes numbers
; only, so the SemVer pre-release suffix is cut off for it. AppVersion keeps the
; full string, which is what people see and what the release is actually called.
#if Pos("-", AppVersion) > 0
  #define NumericVersion Copy(AppVersion, 1, Pos("-", AppVersion) - 1)
#else
  #define NumericVersion AppVersion
#endif

#define AppName "AUVC Capture"
#define AppExe "AUCapture-WPF.exe"

[Setup]
AppId={{9C3F2A41-6E5B-4E77-9C2E-7A1D5B8F0C36}
AppName={#AppName}
AppVersion={#AppVersion}
AppVerName={#AppName} {#AppVersion}
VersionInfoVersion={#NumericVersion}
AppPublisher=AUVC
AppPublisherURL=https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC

; Per user, so installing needs no administrator and no elevation prompt. The
; release contract prefers this, and it is also what lets somebody try AUVC on a
; machine they do not administer.
PrivilegesRequired=lowest
DefaultDirName={localappdata}\Programs\AUVC Capture
DefaultGroupName={#AppName}
DisableProgramGroupPage=yes
DisableDirPage=no

; x64 only: the payload is a self-contained win-x64 build.
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0

OutputDir={#OutputDir}
OutputBaseFilename=AmongUsVoiceCapture-Setup-win-x64
Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern
LicenseFile=..\LICENSE

; Refuse to install over a running capture rather than replacing files in use
; and failing halfway through.
CloseApplications=yes
CloseApplicationsFilter=*.exe
RestartApplications=no

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"
Name: "german"; MessagesFile: "compiler:Languages\German.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; Flags: unchecked

[Files]
Source: "{#PayloadDir}\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{group}\{#AppName}"; Filename: "{app}\{#AppExe}"
Name: "{group}\{cm:UninstallProgram,{#AppName}}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#AppName}"; Filename: "{app}\{#AppExe}"; Tasks: desktopicon

[Run]
Filename: "{app}\{#AppExe}"; Description: "{cm:LaunchProgram,{#AppName}}"; \
    Flags: nowait postinstall skipifsilent

[UninstallDelete]
; Only what the installer created. Settings and the credential live outside
; {app} and are removed separately, and only if asked for.
Type: filesandordirs; Name: "{app}"

[Messages]
english.WelcomeLabel2=This installs {#AppName} for you only. It needs no administrator rights.%n%nAUVC reads Among Us on this PC and runs your Discord bot alongside it. On the first start, a setup window walks you through connecting the bot to your Discord server.
german.WelcomeLabel2=Dies installiert {#AppName} nur für dich. Administratorrechte sind nicht nötig.%n%nAUVC liest Among Us auf diesem PC und betreibt deinen Discord-Bot gleich mit. Beim ersten Start führt dich ein Einrichtungsfenster durch die Verbindung des Bots mit deinem Discord-Server.

[CustomMessages]
english.RemoveDataPrompt=Delete the AUVC data from this PC?%n%nThat is the settings, the stored bot token, the bot's database with channels and player links, its logs, and capture's credential.%n%nThis does not delete the bot in Discord. To stop the token from working, reset it in the Discord Developer Portal.%n%nChoose No to keep everything, so reinstalling needs no setup.
german.RemoveDataPrompt=Sollen die AUVC-Daten von diesem PC gelöscht werden?%n%nDas sind die Einstellungen, der gespeicherte Bot-Token, die Datenbank des Bots mit Kanälen und Spieler-Verknüpfungen, seine Logs und das Credential von Capture.%n%nDer Bot in Discord wird dadurch nicht gelöscht. Damit der Token nicht mehr funktioniert, setze ihn im Discord Developer Portal zurück.%n%nWähle Nein, um alles zu behalten; dann ist nach einer Neuinstallation keine Einrichtung nötig.

[Code]
// Uninstalling asks about local data separately and explicitly, because
// removing a credential from this PC and revoking it in Discord are different
// actions and only the second one actually closes the door.
procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  Settings: string;
  Credential: string;
begin
  if CurUninstallStep <> usPostUninstall then
    Exit;

  if MsgBox(ExpandConstant('{cm:RemoveDataPrompt}'), mbConfirmation, MB_YESNO) <> IDYES then
    Exit;

  Settings := ExpandConstant('{userappdata}\AmongUsCapture');
  Credential := ExpandConstant('{localappdata}\AUVC');

  DelTree(Settings, True, True, True);
  DelTree(Credential, True, True, True);
end;
