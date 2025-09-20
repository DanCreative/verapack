@echo off
goto :main

:checkPermissions (
    net session >nul 2>&1
    if %errorLevel% == 0 (
        set "regKey=HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Control\Session Manager\Environment"
        set "installPath=C:\verapack"
        set "permissionLevel=admin"
    ) else (
        set "regKey=HKEY_CURRENT_USER\Environment"
        set "installPath=%Appdata%\verapack"
        set "permissionLevel=user"
    )

    EXIT /B 0
)
:getRegistryPathEnvironmentValue (
    FOR /F "tokens=3,*" %%g IN ('REG query "%regKey%" /v PATH') do (
        set currPath=%%g%%h
    )

    EXIT /B 0
)
:createDirectory (
    if exist "%installPath%" (
        EXIT /B 0
    )
    MKDIR "%installPath%"

    EXIT /B 0
)
:setEnvPathVariable (
    echo "%currPath%" | findstr /C:"%installPath%" >nul && (
        Exit /B 0
    )

    if "%permissionLevel%" == "admin" (
        SETX /M PATH "%currPath%;%installPath%\;"
    ) else (
        SETX PATH "%currPath%;%installPath%\;"
    )

    EXIT /B 0
)
:installBinary (
    if exist "%~dp0verapack.exe" (
        COPY "%~dp0verapack.exe" "%installPath%\"
    ) else (
        echo file 'verapack.exe' is missing.
        EXIT /B 1
    )

    EXIT /B 0
)
:main (
    call:checkPermissions
    if %errorlevel% neq 0 goto :EOF
    call:getRegistryPathEnvironmentValue
    if %errorlevel% neq 0 goto :EOF
    call:createDirectory
    if %errorlevel% neq 0 goto :EOF
    call:installBinary
    if %errorlevel% neq 0 goto :EOF
    call:setEnvPathVariable
    if %errorlevel% neq 0 goto :EOF
    echo Success
    pause
)