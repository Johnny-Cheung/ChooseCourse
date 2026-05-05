package errno

// 这里定义项目内统一使用的业务错误码。
// 后续学生端、管理员端、抢课业务都应该在这里继续扩展。
const (
	CodeSuccess              = 0
	CodeInvalidParam         = 1001
	CodeUnauthorized         = 1002
	CodeForbidden            = 1003
	CodeInvalidCredentials   = 2001
	CodeUserDisabled         = 2002
	CodeAdminNotFound        = 2003
	CodeStudentNotFound      = 2101
	CodeDuplicateStudentNo   = 2102
	CodeCourseNotFound       = 3001
	CodeInvalidCourseStatus  = 3002
	CodeInvalidCourseCredit  = 3003
	CodeInvalidCourseSlot    = 3004
	CodeInvalidCourseCap     = 3005
	CodeCourseCannotClose    = 3006
	CodeCourseCapTooSmall    = 3007
	CodeCourseLockedFields   = 3008
	CodeCourseClosed         = 3101
	CodeCourseFull           = 3102
	CodeCourseAlreadySelect  = 3103
	CodeCourseTimeConflict   = 3104
	CodeCreditNotEnough      = 3105
	CodeCourseNotSelected    = 3106
	CodeSelectionReqNotFound = 3107
	CodeLikeAlreadyExists    = 4001
	CodeLikeNotFound         = 4002
	CodeCommentNotFound      = 4003
	CodeCommentForbidden     = 4004
	CodeInvalidComment       = 4005
	CodeNotificationNotFound = 4006
	CodeInternalServerError  = 5000
	CodeSystemBusy           = 5001
)

// messages 用来把错误码映射成默认提示语。
var messages = map[int]string{
	CodeSuccess:              "ok",
	CodeInvalidParam:         "invalid parameter",
	CodeUnauthorized:         "unauthorized",
	CodeForbidden:            "forbidden",
	CodeInvalidCredentials:   "user not found or password incorrect",
	CodeUserDisabled:         "user disabled",
	CodeAdminNotFound:        "admin not found",
	CodeStudentNotFound:      "student not found",
	CodeDuplicateStudentNo:   "student number already exists",
	CodeCourseNotFound:       "course not found",
	CodeInvalidCourseStatus:  "invalid course status",
	CodeInvalidCourseCredit:  "invalid course credit",
	CodeInvalidCourseSlot:    "invalid course time slot",
	CodeInvalidCourseCap:     "invalid course capacity",
	CodeCourseCannotClose:    "course cannot be closed after students selected",
	CodeCourseCapTooSmall:    "course capacity cannot be smaller than selected count",
	CodeCourseLockedFields:   "course credit or time slot cannot be changed after students selected",
	CodeCourseClosed:         "course is not open for selection",
	CodeCourseFull:           "course capacity is full",
	CodeCourseAlreadySelect:  "course already selected",
	CodeCourseTimeConflict:   "course time conflict",
	CodeCreditNotEnough:      "remaining credits are not enough",
	CodeCourseNotSelected:    "course not selected yet",
	CodeSelectionReqNotFound: "selection request not found",
	CodeLikeAlreadyExists:    "course already liked",
	CodeLikeNotFound:         "like record not found",
	CodeCommentNotFound:      "comment not found",
	CodeCommentForbidden:     "comment can only be deleted by its owner",
	CodeInvalidComment:       "invalid comment content",
	CodeNotificationNotFound: "notification not found",
	CodeInternalServerError:  "internal server error",
	CodeSystemBusy:           "system busy, please retry later",
}

// Message 根据错误码返回默认文案。
func Message(code int) string {
	if message, ok := messages[code]; ok {
		return message
	}

	return "unknown error"
}
