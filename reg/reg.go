package reg

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Requester struct {
	Url     string
	Cookie  string
	Printed map[string]bool
	course  string
}

func New(cookie, course string) Requester {
	a := Requester{
		Url:     "https://my.sdu.edu.kz/index.php",
		Cookie:  cookie,
		Printed: make(map[string]bool),
		course:  course,
	}
	lec, prac := a.GetCourseInfo()
	if prac != nil {
		for k, v := range lec {
			for _, v := range v.Practices {
				a.Printed[k+v] = false
			}
		}
	} else {
		for k := range lec {
			a.Printed[k] = false
		}
	}
	return a
}

func (r *Requester) CheckAvailable() string {
	fmt.Println("Request made at", time.Now().Format("2006-01-02 15:04:05"))
	lec, prac := r.GetCourseInfo()
	if lec == nil && prac == nil {
		return ""
	}
	if prac == nil {
		return r.HandleWithoutPractice(lec)
	}
	return r.HandleWithPractice(lec, prac)
}

func (r *Requester) HandleWithoutPractice(lec map[string]CourseRight) string {
	var msg strings.Builder
	for k, v := range lec {
		if v.Quota > v.StudCount+v.Reserved && !r.Printed[k] {
			msg.WriteString(fmt.Sprintf("%s\n", v.DersCode))
			msg.WriteString(fmt.Sprintf("  Lecture: %s %s\n", v.Schedule, v.Teacher))
			msg.WriteString("------------------------------------------------------------\n")
			r.Printed[k] = true
		} else if v.Quota == v.StudCount {
			r.Printed[k] = false
		}
	}
	return msg.String()
}

func (r *Requester) HandleWithPractice(lec map[string]CourseRight, prac map[string]CourseRight) string {
	var msg strings.Builder
	for k := range r.Printed {
		lecId := k[0:5]
		pracId := k[5:]
		if lec[lecId].Quota > lec[lecId].StudCount+lec[lecId].Reserved {
			if prac[pracId].Quota > prac[pracId].StudCount && !r.Printed[k] {
				r.Printed[k] = true
				msg.WriteString(fmt.Sprintf("%s\n", lec[lecId].DersCode))
				msg.WriteString(fmt.Sprintf("  Lecture: %s %s\n", lec[lecId].Schedule, lec[lecId].Teacher))
				msg.WriteString(fmt.Sprintf("  Practice: %s %s\n", prac[pracId].Schedule, prac[pracId].Teacher))
				msg.WriteString("------------------------------------------------------------\n")
			} else if prac[pracId].Quota == prac[pracId].StudCount {
				r.Printed[k] = false
			}
		}
	}
	return msg.String()
}

type Course struct {
	DersCode  string `json:"DERS_KOD"`
	Quota     string `json:"QUOTA"`
	StudCount string `json:"STUD_COUNT"`
	Schedule  string `json:"SCHEDULE"`
	Practices string `json:"PRACTICE"`
	Teacher   string `json:"TEACHER"`
	Scheduled string `json:"SCHEDULED"`
	Ders_ID   string `json:"DERS_SOBE_ID"`
	Reserved  string `json:"RESERVED_QUOTA"`
	Type      string `json:"TYPE"`
	Section   string `json:"Type"`
}

type CourseRight struct {
	DersCode  string
	Quota     int
	StudCount int
	Schedule  string
	Practices []string
	Teacher   string
	Scheduled int
	Ders_ID   string
	Reserved  int
}

func (r *Requester) GetCourseInfo() (map[string]CourseRight, map[string]CourseRight) {
	var Lectures map[string]Course
	var Practices map[string]Course
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	url := r.Url

	payload := fmt.Sprintf("ajx=1&mod=course_reg&action=SearchCourse&dk=%s&track=TRACK0&173694081861", r.course)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(payload)))
	if err != nil {
		fmt.Println("Error creating request:", err)
		return nil, nil
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", r.Cookie)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error performing request:", err)
		return nil, nil
	}
	defer resp.Body.Close()

	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			fmt.Println("Error creating gzip reader:", err)
			return nil, nil
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return nil, nil
	}

	body = bytes.TrimPrefix(body, []byte("\xef\xbb\xbf"))

	type Response struct {
		Data []any `json:"DATA2"`
	}
	var gg Response
	err = json.Unmarshal(body, &gg)
	if err != nil {
		fmt.Println("Error unmarshalling JSON:", err)
		return nil, nil
	}
	// fmt.Println(gg.Data...)
	coursedata, _ := json.Marshal(gg.Data[0])
	err = json.Unmarshal(coursedata, &Lectures)
	if err != nil {
		// fmt.Println(err)
		return nil, nil
	}

	coursedata, _ = json.Marshal(gg.Data[1])
	err = json.Unmarshal(coursedata, &Practices)
	if err != nil {
		// fmt.Println(err, "returning lectures only")
		var flag bool
		lect := mapper(Lectures)
		for _, v := range lect {
			if v.Scheduled == 0 {
				flag = true
			}
		}
		if flag {
			return nil, nil
		}
		return lect, nil
	}

	lect := mapper(Lectures)
	prac := mapper(Practices)

	var flag bool
	for _, v := range lect {
		if v.Scheduled == 0 {
			flag = true
		}
	}

	for _, v := range prac {
		if v.Scheduled == 0 {
			flag = true
		}
	}

	if flag {
		return nil, nil
	}

	return lect, prac
}

func mapper(toMap map[string]Course) map[string]CourseRight {
	temp := make(map[string]CourseRight)

	convertCourse := func(c Course) CourseRight {
		quota, _ := strconv.Atoi(c.Quota)
		studCount, _ := strconv.Atoi(c.StudCount)
		practices := strings.Split(c.Practices, ",")
		sch, _ := strconv.Atoi(c.Scheduled)
		reserved, _ := strconv.Atoi(c.Reserved)
		return CourseRight{
			DersCode:  c.DersCode,
			Quota:     quota,
			StudCount: studCount,
			Schedule:  c.Schedule,
			Practices: practices,
			Teacher:   c.Teacher,
			Scheduled: sch,
			Ders_ID:   c.Ders_ID,
			Reserved:  reserved,
		}
	}

	for key, course := range toMap {
		temp[key] = convertCourse(course)
	}

	return temp
}
