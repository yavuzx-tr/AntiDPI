' AntiDPI - Silent Background Launcher for Windows
' Runs AntiDPI without displaying any command prompt or terminal window.

Set WshShell = CreateObject("WScript.Shell")
strDir = CreateObject("Scripting.FileSystemObject").GetParentFolderName(WScript.ScriptFullName)
WshShell.CurrentDirectory = strDir
WshShell.Run """" & strDir & "\run.bat"" -silent", 0, False
