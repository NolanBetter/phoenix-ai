; Phoenix AI installer (NSIS 3).
;
;   makensis -DVERSION=0.2.0 build/installer/phoenix.nsi
;
; Installs for the current user only, so Windows doesn't ask for an
; administrator password. Goes to %LOCALAPPDATA%\Programs\Phoenix AI, adds
; "Phoenix AI" to the Start menu (so Windows search finds it by that name)
; and the desktop, and lists it in Settings > Apps for uninstalling.
;
; Your models and chats live separately in %LOCALAPPDATA%\PhoenixAI and are
; never touched by installing or updating.
;
;   /S        install silently (Phoenix's auto-updater uses this)
;   /UPDATE   open Phoenix again after installing

; A 32-bit installer runs on every Windows PC (the app it installs is
; 64-bit). Builds that have 64-bit stubs can pass -DTARGET=amd64-unicode.
!ifndef TARGET
  !define TARGET "x86-unicode"
!endif
Target ${TARGET}
ManifestDPIAware true
SetCompressor /SOLID lzma
RequestExecutionLevel user

!ifndef VERSION
  !error "Pass the version, for example: makensis -DVERSION=0.2.0 phoenix.nsi"
!endif

!define APP_NAME   "Phoenix AI"
!define APP_EXE    "PhoenixAI.exe"
!define APP_ID     "PhoenixAI"
!define PUBLISHER  "NolanBetter"
!define APP_URL    "https://github.com/NolanBetter/phoenix-ai"
!define UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_ID}"
!define UNINST_EXE "Uninstall Phoenix AI.exe"
!define DATA_DIR   "$LOCALAPPDATA\PhoenixAI"

Name "${APP_NAME}"
OutFile "..\PhoenixAI-Setup-${VERSION}.exe"
InstallDir "$LOCALAPPDATA\Programs\${APP_NAME}"
InstallDirRegKey HKCU "${UNINST_KEY}" "InstallLocation"
BrandingText "${APP_NAME} ${VERSION}"

VIProductVersion "${VERSION}.0"
VIAddVersionKey "ProductName" "${APP_NAME}"
VIAddVersionKey "FileDescription" "${APP_NAME} Setup"
VIAddVersionKey "FileVersion" "${VERSION}"
VIAddVersionKey "ProductVersion" "${VERSION}"
VIAddVersionKey "CompanyName" "${PUBLISHER}"
VIAddVersionKey "LegalCopyright" "${PUBLISHER}"

!include "MUI2.nsh"
!include "LogicLib.nsh"
!include "FileFunc.nsh"

!define MUI_ICON "..\appicon.ico"
!define MUI_UNICON "..\appicon.ico"
!define MUI_WELCOMEFINISHPAGE_BITMAP "welcome.bmp"
!define MUI_UNWELCOMEFINISHPAGE_BITMAP "welcome.bmp"
!define MUI_ABORTWARNING

; ---- pages ----

!define MUI_WELCOMEPAGE_TITLE "Install Phoenix AI"
!define MUI_WELCOMEPAGE_TEXT "Phoenix AI runs open-source AI models right on this PC, so your chats stay private.$\r$\n$\r$\nIt installs just for you and doesn't need administrator permission. Future versions install themselves automatically.$\r$\n$\r$\nClick Install to continue."
!define MUI_PAGE_CUSTOMFUNCTION_SHOW WelcomeShow
!insertmacro MUI_PAGE_WELCOME

!insertmacro MUI_PAGE_INSTFILES

!define MUI_FINISHPAGE_TITLE "Phoenix AI is installed"
!define MUI_FINISHPAGE_TEXT "Open it any time from your desktop or the Start menu. Just search for Phoenix AI."
!define MUI_FINISHPAGE_RUN "$INSTDIR\${APP_EXE}"
!define MUI_FINISHPAGE_RUN_TEXT "Open Phoenix AI now"
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"

Function WelcomeShow
  ; The welcome page leads straight to installing, so call the button Install.
  GetDlgItem $0 $HWNDPARENT 1
  SendMessage $0 ${WM_SETTEXT} 0 "STR:&Install"
FunctionEnd

; ---- wait for Phoenix to close ----
; A running program's file can't be opened for writing. During a silent
; update Phoenix is closing itself, so just wait; otherwise ask.

!macro WaitForAppToClose un
Function ${un}WaitForAppToClose
  StrCpy $R9 0
  check:
    IfFileExists "$INSTDIR\${APP_EXE}" 0 closed
    ClearErrors
    FileOpen $0 "$INSTDIR\${APP_EXE}" a
    IfErrors running
    FileClose $0
    Goto closed
  running:
    IntOp $R9 $R9 + 1
    ${If} ${Silent}
      ${If} $R9 > 60
        SetErrorLevel 2
        Abort
      ${EndIf}
      Sleep 500
      Goto check
    ${EndIf}
    ${If} $R9 < 6
      Sleep 500
      Goto check
    ${EndIf}
    MessageBox MB_RETRYCANCEL|MB_ICONEXCLAMATION "Phoenix AI is still open.$\r$\n$\r$\nClose it, then click Retry." /SD IDCANCEL IDRETRY retry
    Abort
  retry:
    StrCpy $R9 0
    Goto check
  closed:
FunctionEnd
!macroend
!insertmacro WaitForAppToClose ""
!insertmacro WaitForAppToClose "un."

; ---- install ----

Section "Install"
  SetShellVarContext current
  Call WaitForAppToClose

  SetOutPath "$INSTDIR"
  File "..\PhoenixAI.exe"
  WriteUninstaller "$INSTDIR\${UNINST_EXE}"

  ; Shortcuts are named "Phoenix AI", which is what Windows search shows.
  CreateShortCut "$SMPROGRAMS\${APP_NAME}.lnk" "$INSTDIR\${APP_EXE}" "" "$INSTDIR\${APP_EXE}" 0 SW_SHOWNORMAL "" "Run open-source AI models on this PC"
  ; Silent updates only refresh the desktop shortcut if it's still there,
  ; so one you deleted doesn't come back.
  ${If} ${Silent}
    ${If} ${FileExists} "$DESKTOP\${APP_NAME}.lnk"
      CreateShortCut "$DESKTOP\${APP_NAME}.lnk" "$INSTDIR\${APP_EXE}" "" "$INSTDIR\${APP_EXE}" 0 SW_SHOWNORMAL "" "Run open-source AI models on this PC"
    ${EndIf}
  ${Else}
    CreateShortCut "$DESKTOP\${APP_NAME}.lnk" "$INSTDIR\${APP_EXE}" "" "$INSTDIR\${APP_EXE}" 0 SW_SHOWNORMAL "" "Run open-source AI models on this PC"
  ${EndIf}

  ; Settings > Apps entry.
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayName" "${APP_NAME}"
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKCU "${UNINST_KEY}" "Publisher" "${PUBLISHER}"
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayIcon" "$INSTDIR\${APP_EXE},0"
  WriteRegStr HKCU "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${UNINST_KEY}" "UninstallString" '"$INSTDIR\${UNINST_EXE}"'
  WriteRegStr HKCU "${UNINST_KEY}" "QuietUninstallString" '"$INSTDIR\${UNINST_EXE}" /S'
  WriteRegStr HKCU "${UNINST_KEY}" "URLInfoAbout" "${APP_URL}"
  WriteRegDWORD HKCU "${UNINST_KEY}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINST_KEY}" "NoRepair" 1
  ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
  IntFmt $0 "0x%08X" $0
  WriteRegDWORD HKCU "${UNINST_KEY}" "EstimatedSize" "$0"

  ; After an update, open Phoenix again.
  ${GetParameters} $R0
  ClearErrors
  ${GetOptions} $R0 "/UPDATE" $R1
  ${IfNot} ${Errors}
    Exec '"$INSTDIR\${APP_EXE}"'
  ${EndIf}
SectionEnd

; ---- uninstall ----

Section "Uninstall"
  SetShellVarContext current
  Call un.WaitForAppToClose

  Delete "$INSTDIR\${APP_EXE}"
  Delete "$INSTDIR\${UNINST_EXE}"
  RMDir "$INSTDIR"
  Delete "$SMPROGRAMS\${APP_NAME}.lnk"
  Delete "$DESKTOP\${APP_NAME}.lnk"
  DeleteRegKey HKCU "${UNINST_KEY}"
  ; What Windows notifications used to find Phoenix.
  DeleteRegKey HKCU "Software\Classes\AppUserModelId\Phoenix AI"
  DeleteRegKey HKCU "Software\Classes\CLSID\{6E3B1F4A-92C8-4D57-A0B3-7C1E5D9F2A68}"

  ; Models and chats stay unless you say otherwise (they can be big
  ; downloads, and you might reinstall).
  ${IfNot} ${Silent}
    MessageBox MB_YESNO|MB_ICONQUESTION|MB_DEFBUTTON2 "Also delete your downloaded models, chats and settings?$\r$\n$\r$\nChoose No to keep them in case you install Phoenix AI again." IDNO keep
    RMDir /r "${DATA_DIR}"
    keep:
  ${EndIf}
SectionEnd
