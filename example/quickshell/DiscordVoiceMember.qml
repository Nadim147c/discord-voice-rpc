import QtQuick

// This component is used to store the data of a voice member.

QtObject {
    property string id: ""
    property string username: ""
    property string nickname: ""
    property string serverName: ""
    property string avatar: ""
    property string avatarURL: ""
    property bool isTalking: false
    property bool isBot: false
    property bool isMute: false
    property bool isDeaf: false
    property bool isSelfDeaf: false
    property bool isSelfMute: false
    property bool isSuppressed: false
}
