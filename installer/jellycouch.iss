; JellyCouch Windows installer (Inno Setup 6)
; Build with: iscc installer\jellycouch.iss
; Or use installer\build.ps1 for a one-shot build.

#ifndef MyAppVersion
  #define MyAppVersion "0.1.0"
#endif

#define MyAppName       "JellyCouch"
#define MyAppPublisher  "JellyCouch"
#define MyAppURL        "https://github.com/depeter/jellycouch"
#define MyAppExeName    "jellycouch.exe"
; Paths are resolved relative to this .iss file.
#define SourceRoot      "..\"
#define IconFile        "jellycouch.ico"

[Setup]
AppId={{4328EBEF-AE51-4134-AE9C-5E7A306079E4}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppVerName={#MyAppName} {#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}
VersionInfoVersion={#MyAppVersion}
DefaultDirName={autopf}\{#MyAppName}
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
OutputDir={#SourceRoot}dist
OutputBaseFilename=JellyCouch-Setup-{#MyAppVersion}
; libmpv.dll is x64-only.
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
; Let the user choose per-user or system-wide.
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog commandline
Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern
UninstallDisplayIcon={app}\{#MyAppExeName}
SetupIconFile={#IconFile}
; Don't close the app if user is mid-install without asking.
CloseApplications=yes
RestartApplications=no

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
Source: "{#SourceRoot}jellycouch.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceRoot}libmpv.dll";     DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceRoot}README.md";      DestDir: "{app}"; Flags: ignoreversion isreadme

[Icons]
Name: "{group}\{#MyAppName}";          Filename: "{app}\{#MyAppExeName}"
Name: "{group}\Uninstall {#MyAppName}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#MyAppName}";    Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#MyAppName}}"; Flags: nowait postinstall skipifsilent

[UninstallDelete]
; Leave user config (%APPDATA%\jellycouch) alone on uninstall — matches standard app behavior.
