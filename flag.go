package main

type StringArray []string

func (av *StringArray) String() string {
	return ""
}

func (av *StringArray) Set(s string) error {
	*av = append(*av, s)
	return nil
}
