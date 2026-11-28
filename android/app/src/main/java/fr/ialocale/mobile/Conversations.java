package fr.ialocale.mobile;

import android.content.ContentValues;
import android.content.Context;
import android.database.Cursor;
import android.database.sqlite.SQLiteDatabase;
import android.database.sqlite.SQLiteOpenHelper;
import java.util.ArrayList;

final class Conversations extends SQLiteOpenHelper {
    Conversations(Context context) { super(context,"discussions.db",null,1); }
    public void onCreate(SQLiteDatabase db) {
        db.execSQL("CREATE TABLE turns (id INTEGER PRIMARY KEY, category TEXT, thread TEXT, prompt TEXT, answer TEXT)");
    }
    public void onUpgrade(SQLiteDatabase db,int oldVersion,int newVersion) {}
    void add(String category,String thread,String prompt,String answer) {
        ContentValues values=new ContentValues();
        values.put("category",category); values.put("thread",thread);
        values.put("prompt",prompt); values.put("answer",answer);
        getWritableDatabase().insertOrThrow("turns",null,values);
    }
    ArrayList<String[]> list(String category) {
        ArrayList<String[]> rows=new ArrayList<>();
        try(Cursor c=getReadableDatabase().rawQuery("SELECT thread,prompt FROM turns WHERE category=? AND id IN (SELECT MIN(id) FROM turns GROUP BY thread) ORDER BY id DESC",new String[]{category})) {
            while(c.moveToNext())rows.add(new String[]{c.getString(0),c.getString(1)});
        }
        return rows;
    }
    String read(String thread) {
        StringBuilder text=new StringBuilder();
        try(Cursor c=getReadableDatabase().rawQuery("SELECT prompt, answer FROM turns WHERE thread=? ORDER BY id",new String[]{thread})) {
            while(c.moveToNext())text.append("Vous · ").append(c.getString(0)).append("\nStudio · ").append(c.getString(1)).append("\n\n");
        }
        return text.toString();
    }
    void remove(String thread) {getWritableDatabase().delete("turns","thread=?",new String[]{thread});}
}
