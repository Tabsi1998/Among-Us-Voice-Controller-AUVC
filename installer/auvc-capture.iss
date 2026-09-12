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
english.WelcomeLabel2=This installs {#AppName} for you only. It needs no administrator rights.%n%nAUVC is two programs: this one reads the game on this PC, and a bot talks to Discord. You will pair them with a code from /au capture pair.
german.WelcomeLabel2=Dies installiert {#AppName} nur für dich. Administratorrechte sind nicht nötig.%n%nAUVC besteht aus zwei Programmen: Dieses liest das Spiel auf diesem PC, ein Bot spricht mit Discord. Beide werden mit einem Code aus /au capture pair verbunden.

[CustomMessages]
english.RemoveDataPrompt=Delete the AUVC capture settings and the paired credential from this PC?%n%nThis does not revoke access. The bot still accepts this credential until somebody runs /au capture revoke in Discord.%n%nChoose No to keep them, so reinstalling does not need pairing again.
german.RemoveDataPrompt=Sollen die AUVC-Capture-Einstellungen und das gekoppelte Credential von diesem PC gelöscht werden?%n%nDas widerruft den Zugriff nicht. Der Bot akzeptiert dieses Credential weiterhin, bis jemand in Discord /au capture revoke ausführt.%n%nWähle Nein, um sie zu behalten; dann ist nach einer Neuinstallation kein erneutes Koppeln nötig.

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
