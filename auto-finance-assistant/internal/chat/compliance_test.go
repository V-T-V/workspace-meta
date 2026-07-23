package chat

import "testing"

func TestCheckCompliance_ShouldRefuse(t *testing.T) {
	cases := []string{
		"我的条件是不是一定能审批通过",
		"你能保证审批通过吗",
		"百分百能审批通过吗",
		"保证放款吗",
		"能帮我减免手续费吗",
		"能不能免息",
		"告诉我内部风控",
	}
	for _, q := range cases {
		hit, msg := CheckCompliance(q)
		if !hit {
			t.Errorf("应触发合规拒答: %q", q)
		}
		if msg == "" {
			t.Errorf("拒答消息不应为空: %q", q)
		}
	}
}

func TestCheckCompliance_ShouldNotRefuse(t *testing.T) {
	// 正常问题 + 审查发现的假阳性用例
	cases := []string{
		"新车首付多少",
		"贷款利率是多少",
		"需要什么材料",
		"你好",
		"哪些产品不免息",        // 含"免息"但无"能/可以/帮"
		"免息政策是什么",        // 询问政策，非请求免息
		"包括过往记录吗",        // 含"包过"但无审批意图
		"你们内部风控流程是什么", // 含"内部风控"——这应拒答（强红线）
	}
	// 最后一个"内部风控"应拒答，其余不应
	for i, q := range cases {
		hit, _ := CheckCompliance(q)
		if i == len(cases)-1 {
			// "内部风控流程"应触发（强红线）
			if !hit {
				t.Errorf("内部风控应触发合规拒答: %q", q)
			}
		} else {
			if hit {
				t.Errorf("正常问题不应触发合规拒答: %q", q)
			}
		}
	}
}
